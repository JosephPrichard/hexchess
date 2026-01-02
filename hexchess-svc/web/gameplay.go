package web

import (
	"context"
	"errors"
	"fmt"
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
}

func makeGameErr(ctx context.Context, gameID string, err error) []byte {
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
	return bytes
}

func makeGameInitErr(ctx context.Context, gameID string, err error) []byte {
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
		slog.ErrorContext(ctx, "failed to marshal err output", "err", err)
		bytes = nil
	}
	return bytes
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
		return
	}

	player, err := app.handleGameInit(ctx, gameID, sessionID, func(v []byte) { writeConn(ctx, conn, v) })
	if err != nil {
		writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
		return
	}
	defer conn.Close()

	writeChan := make(chan []byte, GameplayChanBufCap)
	app.GamesCaster.Subscribe(gameID, writeChan)

	go func() {
		for b := range writeChan {
			writeConn(ctx, conn, b)
		}
	}()

	defer app.GamesCaster.Unsubscribe(gameID, writeChan)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "err", err)
			break
		}
		go app.handleGameMessage(GameSocketContext{Context: ctx, GameID: gameID, Player: player}, msg, writeChan)
	}
}

func (app *App) handleGameInit(ctx context.Context, gameID string, sessionID string, write func([]byte)) (svc.PlayerState, error) {
	var p svc.PlayerState

	p, err := app.State.GetSession(ctx, sessionID)
	if err != nil {
		return p, err
	}
	state, err := app.State.JoinGame(ctx, gameID, p)
	if err != nil {
		return p, err
	}

	for i, o := range []*pb.GameOutput{
		MakePbGameOutputInit(
			gameID,
			svc.SerializeChessState(state),
			svc.SerializePlayer(p),
		),
		MakePbGameOutputPlayers(
			gameID,
			svc.SerializePlayer(state.WhitePlayer),
			svc.SerializePlayer(state.BlackPlayer),
		),
	} {
		b, err := proto.Marshal(o)
		if err != nil {
			return p, fmt.Errorf("marshal game init output %d: %w", i, err)
		}
		write(b)
	}
	return p, nil
}

func (app *App) handleGameMessage(ctx GameSocketContext, msg []byte, writeChan chan []byte) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
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
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
	}
}

func (app *App) handleGameForfeit(ctx GameSocketContext) error {
	replayID, err := app.State.ForfeitGame(ctx.Context, ctx.GameID, ctx.Player)
	if err != nil {
		return err
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(ctx.GameID, replayID))
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	result, err := app.State.MakeGameMove(ctx.Context, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputMove(
		ctx.GameID,
		chess.SerializeHistMove(result.Move),
		chess.SerializeGame(&result.State.Game),
		app.GetNow(),
	))
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}

func (app *App) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	bytes, err := proto.Marshal(MakePbGameOutputChat(ctx.GameID, pbInput.Message, svc.SerializePlayer(ctx.Player)))
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
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID,
		state,
	))
	if err != nil {
		return err
	}
	return app.State.BroadcastGamesEvent(ctx.Context, bytes)
}
