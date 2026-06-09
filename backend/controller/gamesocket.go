package controller

import (
	"context"
	"errors"
	"hexchess-svc/chess"
	"hexchess-svc/lib/logutil"

	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/pb"
	svc "hexchess-svc/service"

	"github.com/gorilla/websocket"
)

type GameSocketContext struct {
	Context context.Context
	GameID  string
	Player  model.PlayerState
	ErrChan chan error
}

// GameplayChanBufCap start dropping messages when a websocket is behind by this many messages
const GameplayChanBufCap = 10

func (api *API) HandleGameWs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "Err", err)
		return
	}
	defer conn.Close()

	// subscribe before we begin init, so the number of messages we expect as a result of the initialization stage is deterministic.
	subscriber := make(chan message, GameplayChanBufCap)
	api.broadcasters.GamesCaster.Subscribe(gameID, subscriber)
	defer api.broadcasters.GamesCaster.Unsubscribe(gameID, subscriber)

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

	// begin the init phase, which retrieves state and writes back to clients
	player, err := api.handleGameInit(ctx, gameID, sessionID, conn)
	if err != nil {
		// write an error and close if we run into any issues. init is idempotent so the client can retry until everything works
		writeGameInitErr(ctx, conn, gameID, err)
		return
	}

	// read and handle each input message, with each handler running concurrently
	gameSocketCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: player, ErrChan: errChan}
	for {
		_, input, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "Err", err)
			break
		}
		go api.handleGameMessage(gameSocketCtx, input)
	}

	slog.InfoContext(ctx, "gameplay websocket reader closed", "gameID", gameID)
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
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, svc.ErrUndoCurrPlayer):
		wsErr = ErrWsUndoCurrPlayer
	case errors.Is(err, model.ErrNoMoveUndo), errors.Is(err, svc.ErrUndoNoop), errors.Is(err, svc.ErrNoUndo):
		wsErr = ErrWsUndoAction
	}

	logutil.RootLog(ctx, slog.LevelError, "failed to handle ws message", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(SerializeGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal Err output", "Err", err)
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

	logutil.RootLog(ctx, slog.LevelError, "failed to initialize gameplay websocket", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(SerializeGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal init Err output", "Err", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func (api *API) handleGameInit(ctx context.Context, gameID string, sessionID string, conn *websocket.Conn) (player model.PlayerState, err error) {
	// apply state updates for the init phase
	player, err = api.services.GetSession(ctx, sessionID)
	if err != nil {
		return player, serrors.New("get session in game init phase", err)
	}
	chessState, err := api.services.JoinGame(ctx, gameID, player)
	if err != nil {
		return player, serrors.New("join game in init game phase", err)
	}

	// produce messages for init phase
	initBytes, err := proto.Marshal(SerializeGameOutputInit(
		gameID,
		model.SerializeChessState(chessState),
		model.SerializePlayer(player),
	))
	if err != nil {
		return player, serrors.New("marshal init output", err)
	}
	writeMessage(ctx, conn, initBytes)

	api.broadcaster.BroadcastGamesEvent(ctx, SerializeGameOutputPlayers(
		gameID,
		model.SerializePlayer(chessState.WhitePlayer),
		model.SerializePlayer(chessState.BlackPlayer),
	))

	return player, nil
}

func (api *API) handleGameMessage(ctx GameSocketContext, input message) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(input, &pbInput); err != nil {
		ctx.ErrChan <- err
		return
	}

	slog.InfoContext(ctx.Context, "received game input", "pbInput", &pbInput)

	var err error
	switch p := pbInput.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		err = api.handleGameForfeit(ctx)
	case *pb.GameInput_Move:
		err = api.handleGameMove(ctx, p.Move)
	case *pb.GameInput_Chat:
		err = api.handleGameChat(ctx, p.Chat)
	case *pb.GameInput_Undo:
		err = api.handleGameUndo(ctx, p.Undo)
	case *pb.GameInput_Ping:
		// no-op or heartbeat
	default:
		err = ErrWsMessageType
	}
	if err != nil {
		ctx.ErrChan <- err
	}
}

func (api *API) handleGameForfeit(ctx GameSocketContext) error {
	endState, err := api.services.EndGame(ctx.Context, ctx.GameID, ctx.Player)
	if err != nil {
		return serrors.New("forfeit game", err, "gameID", ctx.GameID)
	}
	api.broadcaster.BroadcastGamesEvent(ctx.Context, SerializeGameOutputForfeit(ctx.GameID, endState))
	return nil
}

func (api *API) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	moveResult, err := api.services.MakeGameMove(ctx.Context, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return serrors.New("make move on game", err, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx.Context, SerializeGameOutputMove(
		ctx.GameID,
		chess.SerializeHistMove(moveResult.Move),
		chess.SerializeGame(&moveResult.State.Game),
		time.Now(),
	))
	return nil
}

func (api *API) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	chatMsg := model.Chat{
		ID:      uuid.NewString(),
		Player:  ctx.Player,
		Message: pbInput.Message,
		SentAt:  time.Now(),
	}
	outputChat := SerializeGameOutputChat(ctx.GameID, chatMsg)

	if err := api.services.InsertChat(ctx.Context, ctx.GameID, chatMsg); err != nil {
		return serrors.New("insert chat on game", err, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx.Context, outputChat)
	return nil
}

func (api *API) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput) error {
	undoKind, err := DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	state, err := api.services.AttemptGameUndo(ctx.Context, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return serrors.New("attempting undo on game", err, "player", ctx.Player, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx.Context, SerializeGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID,
		state,
	))
	return nil
}
