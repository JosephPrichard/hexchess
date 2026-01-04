package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
)

type GameplayHandler struct {
	*svc.State
	svc.LocalBroadcasters
}

type GameSocketContext struct {
	Context context.Context
	GameID  string
	Player  svc.PlayerState
	ErrChan chan error
}

func writeGameMsgErr(ctx context.Context, conn *websocket.Conn, gameID string, err error) {
	var wsErr error
	switch {
	case errors.Is(err, svc.ErrFinishedGame):
		wsErr = ErrWsFinishedGame
	case errors.Is(err, svc.ErrTurn):
		wsErr = ErrWsTurn
	case errors.Is(err, svc.ErrInvalidMove):
		wsErr = ErrWsInvalidMove
	case errors.Is(err, svc.ErrNoChessState):
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, svc.ErrNoMoveUndo) || errors.Is(err, svc.ErrUndoNoop) || errors.Is(err, svc.ErrNoUndo):
		wsErr = ErrWsUndoAction
	default:
		wsErr = ErrWsFatal
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

	initResult, err := app.handleGameInit(ctx, gameID, sessionID)
	if err != nil {
		writeGameInitErr(ctx, conn, gameID, err) // write an error and close if we run into any issues. init is idempotent so the client can retry until everything works
		return
	}
	for _, v := range initResult.Messages {
		writeMessage(ctx, conn, v)
	}

	// subscribe before we send the broadcast, so the number of messages we expect as a result of the initialization stage is deterministic.
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

	for _, v := range initResult.Broadcasts {
		if err := app.State.BroadcastGamesEvent(ctx, v); err != nil {
			writeGameInitErr(ctx, conn, gameID, err) // if we fail to write a broadcast event for any reason, we cannot proceed since other clients don't have the correct state
			return
		}
	}
	go app.handleGameBgInit(ctx, gameID)

	wsCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: initResult.Player, ErrChan: errChan}
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

type InitResult struct {
	Player     svc.PlayerState
	Messages   []message
	Broadcasts []message
}

func (app *App) handleGameInit(ctx context.Context, gameID string, sessionID string) (r InitResult, err error) {
	// apply state updates for the init phase
	player, err := app.State.GetSession(ctx, sessionID)
	if err != nil {
		return r, fmt.Errorf("get session in game init phase: %w", err)
	}
	state, err := app.State.JoinGame(ctx, gameID, player)
	if err != nil {
		return r, fmt.Errorf("join game in init game phase: %w", err)
	}

	// produce messages for init phase
	initBytes, err := proto.Marshal(MakePbGameOutputInit(
		gameID,
		svc.SerializeChessState(state),
		svc.SerializePlayer(player),
	))
	if err != nil {
		return r, fmt.Errorf("marshal init output: %w", err)
	}

	playersBytes, err := proto.Marshal(MakePbGameOutputPlayers(
		gameID,
		svc.SerializePlayer(state.WhitePlayer),
		svc.SerializePlayer(state.BlackPlayer),
	))
	if err != nil {
		return r, fmt.Errorf("marshal players output: %w", err)
	}

	r.Player = player
	r.Messages = append(r.Messages, initBytes)
	r.Broadcasts = append(r.Broadcasts, playersBytes)

	return r, nil
}

func (app *App) handleGameBgInit(ctx context.Context, gameID string) {
	chats, err := app.State.GetStateChats(ctx, gameID, 100)
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

	if err := app.State.BroadcastGamesEvent(ctx, bytes); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast bg init output", "err", err)
	}
}

func (app *App) handleGameMessage(ctx GameSocketContext, input message) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(input, &pbInput); err != nil {
		ctx.ErrChan <- err
		return
	}

	slog.InfoContext(ctx.Context, "received game input", "pbInput", &pbInput)

	var err error
	if f := pbInput.GetForfeit(); f != nil {
		err = app.handleGameForfeit(ctx)
	} else if m := pbInput.GetMove(); m != nil {
		err = app.handleGameMove(ctx, m)
	} else if c := pbInput.GetChat(); c != nil {
		err = app.handleGameChat(ctx, c)
	} else if u := pbInput.GetUndo(); u != nil {
		err = app.handleGameUndo(ctx, u)
	} else {
		err = ErrWsMessageType
	}
	if err != nil {
		ctx.ErrChan <- err
	}
}

func (app *App) handleGameForfeit(ctx GameSocketContext) error {
	cs, err := app.State.ForfeitGame(ctx.Context, ctx.GameID, ctx.Player)
	if err != nil {
		return fmt.Errorf("forfeit game %s: %w", ctx.GameID, err)
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(ctx.GameID, cs.ReplayID))
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	result, err := app.State.MakeGameMove(ctx.Context, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
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
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	outputChat, chatMsg := MakePbGameOutputChat(
		ctx.GameID,
		pbInput.Message,
		ctx.Player,
		app.GetNow(),
	)
	if err := app.State.InsertStateChat(ctx.Context, ctx.GameID, chatMsg); err != nil {
		return fmt.Errorf("insert chat on game %s: %w", ctx.GameID, err)
	}
	bytes, err := proto.Marshal(outputChat)
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
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

	state, err := app.State.AttemptGameUndo(ctx.Context, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return fmt.Errorf("attempting undo on game %s: %w", ctx.GameID, err)
	}

	bytes, err := proto.Marshal(MakePbGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID, state,
	))
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}
