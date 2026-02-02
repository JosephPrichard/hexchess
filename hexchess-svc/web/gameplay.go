package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/errutil"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
)

type GameSocketContext struct {
	Context context.Context
	GameID  string
	Player  svc.PlayerState
	ErrChan chan error
}

func writeGameMsgErr(ctx context.Context, conn *websocket.Conn, gameID string, err error) {
	wsErr := ErrWsFatal
	switch {
	case errutil.IsType[svc.ErrFinishedGame](err):
		wsErr = ErrWsFinishedGame
	case errors.Is(err, svc.ErrForfeitPlayer):
		wsErr = ErrWsForfeitPlayer
	case errutil.IsType[svc.ErrStartedGame](err):
		wsErr = ErrWsStartedGame
	case errutil.IsType[svc.ErrTurn](err):
		wsErr = ErrWsTurn
	case errutil.IsType[svc.ErrInvalidMove](err):
		wsErr = ErrWsInvalidMove
	case errors.Is(err, svc.ErrNoChessState):
		// if the s cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, svc.ErrUndoCurrPlayer):
		wsErr = ErrWsUndoCurrPlayer
	case errors.Is(err, svc.ErrNoMoveUndo), errors.Is(err, svc.ErrUndoNoop), errors.Is(err, svc.ErrNoUndo):
		wsErr = ErrWsUndoAction
	}

	slog.WarnContext(ctx, "failed to handle ws message", "err", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(MakePbGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal err output", "err", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func writeGameInitErr(ctx context.Context, conn *websocket.Conn, gameID string, err error) {
	var wsErr error
	switch {
	case errors.Is(err, svc.ErrNoChessState):
		wsErr = ErrWsInvalidGame
	default:
		wsErr = ErrWsFatal
	}
	slog.WarnContext(ctx, "failed to initialize gameplay websocket", "err", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(MakePbGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal init err output", "err", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

// GameplayChanBufCap start dropping messages when a websocket is behind by this many messages
const GameplayChanBufCap = 10

func (app *App) HandleGameWs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "err", err)
		return
	}
	defer conn.Close()

	// subscribe before we begin init, so the number of messages we expect as a result of the initialization stage is deterministic.
	subscriber := make(chan message, GameplayChanBufCap)
	app.GamesCaster.Subscribe(gameID, subscriber)
	defer app.GamesCaster.Unsubscribe(gameID, subscriber)

	errChan := make(chan error)

	go func() {
		// write back broadcasts (from subscriber) and errors (from input messages) back to the client
	RecvLoop:
		for {
			select {
			case err := <-errChan:
				writeGameMsgErr(ctx, conn, gameID, err)
			case v, ok := <-subscriber:
				if !ok {
					break RecvLoop
				}
				writeMessage(ctx, conn, v)
			}
		}
		conn.Close()
		slog.InfoContext(ctx, "gameplay websocket writer closed", "gameID", gameID)
	}()

	// begin the init phase, which retrieves s and writes back to clients
	player, err := app.handleGameInit(ctx, gameID, sessionID, conn)
	if err != nil {
		writeGameInitErr(ctx, conn, gameID, err) // write an error and close if we run into any issues. init is idempotent so the client can retry until everything works
		return
	}

	// read and handle each input message, with each handler running concurrently
	wsCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: player, ErrChan: errChan}
	for {
		_, input, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "err", err)
			break
		}
		go app.handleGameMessage(wsCtx, input)
	}

	slog.InfoContext(ctx, "gameplay websocket reader closed", "gameID", gameID)
}

func (app *App) handleGameInit(ctx context.Context, gameID string, sessionID string, conn *websocket.Conn) (player svc.PlayerState, err error) {
	// apply s updates for the init phase
	player, err = app.Services.GetSession(ctx, sessionID)
	if err != nil {
		return player, fmt.Errorf("get session in game init phase: %w", err)
	}
	s, err := app.Services.JoinGame(ctx, gameID, player)
	if err != nil {
		return player, fmt.Errorf("join game in init game phase: %w", err)
	}

	// produce messages for init phase
	initBytes, err := proto.Marshal(MakePbGameOutputInit(
		gameID,
		svc.SerializeChessState(s),
		svc.SerializePlayer(player),
	))
	if err != nil {
		return player, fmt.Errorf("marshal init output: %w", err)
	}

	playersBytes, err := proto.Marshal(MakePbGameOutputPlayers(
		gameID,
		svc.SerializePlayer(s.WhitePlayer),
		svc.SerializePlayer(s.BlackPlayer),
	))
	if err != nil {
		return player, fmt.Errorf("marshal players output: %w", err)
	}

	writeMessage(ctx, conn, initBytes)
	if err := app.Services.BroadcastGamesEvent(ctx, playersBytes); err != nil {
		return player, err
	}
	go func() {
		// we load this in the background because we don't care if for whatever reason we can't load the chat.
		chats, err := app.Services.GetStateChats(ctx, gameID, 100)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get game chats for broadcast", "err", err)
			return
		}

		bytes, err := proto.Marshal(MakePbGameOutputBgInit(
			gameID,
			svc.SerializeChats(chats),
		))
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal bg init output", "err", err)
			return
		}

		if err := app.Services.BroadcastGamesEvent(ctx, bytes); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast bg init output", "err", err)
		}
	}()

	return player, nil
}

func (app *App) handleGameMessage(ctx GameSocketContext, input message) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(input, &pbInput); err != nil {
		ctx.ErrChan <- err
		return
	}

	slog.InfoContext(ctx.Context, "received game input", "pbInput", &pbInput)

	var err error
	switch p := pbInput.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		err = app.handleGameForfeit(ctx)
	case *pb.GameInput_Move:
		err = app.handleGameMove(ctx, p.Move)
	case *pb.GameInput_Chat:
		err = app.handleGameChat(ctx, p.Chat)
	case *pb.GameInput_Undo:
		err = app.handleGameUndo(ctx, p.Undo)
	case *pb.GameInput_Ping:
		// no-op or heartbeat
	default:
		err = ErrWsMessageType
	}
	if err != nil {
		ctx.ErrChan <- err
	}
}

func (app *App) handleGameForfeit(ctx GameSocketContext) error {
	replayID, fs, err := app.Services.ForfeitGame(ctx.Context, ctx.GameID, ctx.Player)
	if err != nil {
		return fmt.Errorf("forfeit game %s: %w", ctx.GameID, err)
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(
		ctx.GameID,
		replayID,
		svc.SerializeEndState(fs),
	))
	if err != nil {
		return err
	}
	return app.Services.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	result, err := app.Services.MakeGameMove(ctx.Context, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return fmt.Errorf("make move on game %s: %w", ctx.GameID, err)
	}

	bytes, err := proto.Marshal(MakePbGameOutputMove(
		ctx.GameID,
		chess.SerializeHistMove(result.Move),
		chess.SerializeGame(&result.State.Game), app.GetNow(),
	))
	if err != nil {
		return err
	}
	return app.Services.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	outputChat, chatMsg := MakePbGameOutputChat(
		ctx.GameID,
		pbInput.Message,
		ctx.Player,
		app.GetNow(),
	)
	if err := app.Services.InsertStateChat(ctx.Context, ctx.GameID, chatMsg); err != nil {
		return fmt.Errorf("insert chat on game %s: %w", ctx.GameID, err)
	}
	bytes, err := proto.Marshal(outputChat)
	if err != nil {
		return err
	}
	return app.Services.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput) error {
	var undoKind svc.UndoKind
	switch pbInput.Kind {
	case "CREATE":
		undoKind = svc.UndoCreate
	case "ACCEPT":
		undoKind = svc.UndoAccept
	case "REJECT":
		undoKind = svc.UndoReject
	default:
		return fmt.Errorf("invalid undo kind: %s", pbInput.Kind)
	}

	s, err := app.Services.AttemptGameUndo(ctx.Context, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return fmt.Errorf("%+v attempting undo on game %s: %w", ctx.Player, ctx.GameID, err)
	}

	bytes, err := proto.Marshal(MakePbGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID, s,
	))
	if err != nil {
		return err
	}
	return app.Services.BroadcastGamesEvent(ctx.Context, bytes)
}
