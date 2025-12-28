package web

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pb"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
)

type GameplayHandler struct {
	db.Databases
	svc.Broadcasters
	outbound.Generators
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
	default:
		wsErr = ErrWsFatal
	}

	slog.WarnContext(ctx, "failed to error occurred while handling ws message", "err", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(MakePbGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal err output sseMsgData", "err", err)
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
	slog.WarnContext(ctx, "failed to error occurred in initializing gameplay websocket", "err", err, "wsErr", wsErr)
	return makeGameErr(ctx, gameID, wsErr)
}

// GameplayChanBufCap start dropping messages when a websocket is behind by this many messages
const GameplayChanBufCap = 10

func (h *GameplayHandler) HandleGameWs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	player, err := svc.GetSession(ctx, h.Rdb, sessionID)
	if err != nil {
		writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
		return
	}
	chessState, err := svc.JoinGame(ctx, h.Rdb, gameID, player)
	if err != nil {
		writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
		return
	}

	if err := h.handleGameInit(gameID, player, chessState, func(b []byte) {
		writeConn(ctx, conn, b)
	}); err != nil {
		slog.ErrorContext(ctx, "failed to handle game init", "err", err)
		return
	}

	gameCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: player}

	writeChan := make(chan []byte, GameplayChanBufCap)
	h.GamesCaster.Subscribe(gameID, writeChan)

	go func() {
		for b := range writeChan {
			writeConn(ctx, conn, b)
		}
	}()

	defer h.GamesCaster.Unsubscribe(gameID, writeChan)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "err", err)
			break
		}
		go h.handleGameMessage(gameCtx, msg, writeChan)
	}
}

func (h *GameplayHandler) handleGameInit(gameID string, player svc.PlayerState, state *svc.ChessState, write func([]byte)) error {
	for i, o := range []*pb.GameOutput{
		MakePbGameOutputInit(
			gameID,
			svc.SerializeChessState(state),
			svc.SerializePlayer(player),
		),
		MakePbGameOutputPlayers(
			gameID,
			svc.SerializePlayer(state.WhitePlayer),
			svc.SerializePlayer(state.BlackPlayer),
		),
	} {
		b, err := proto.Marshal(o)
		if err != nil {
			return fmt.Errorf("marshal game init output %d: %w", i, err)
		}
		write(b)
	}
	return nil
}

func (h *GameplayHandler) handleGameMessage(ctx GameSocketContext, msg []byte, writeChan chan []byte) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
		return
	}

	slog.InfoContext(ctx.Context, "received game input", "pbInput", &pbInput)

	var err error
	if f := pbInput.GetForfeit(); f != nil {
		err = h.handleGameForfeit(ctx)
	} else if m := pbInput.GetMove(); m != nil {
		err = h.handleGameMove(ctx, m)
	} else if c := pbInput.GetChat(); c != nil {
		err = h.handleGameChat(ctx, c)
	} else if u := pbInput.GetUndo(); u != nil {
		err = h.handleGameUndo(ctx, u)
	} else {
		err = ErrWsMessageType
	}

	if err != nil {
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
	}
}

func (h *GameplayHandler) handleGameForfeit(ctx GameSocketContext) error {
	if err := svc.ForfeitGame(ctx.Context, &h.Databases, ctx.GameID, ctx.Player); err != nil {
		return err
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(ctx.GameID))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, h.Rdb, h.Rdb.GamesChan, bytes)
}

func (h *GameplayHandler) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	result, err := svc.MakeGameMove(ctx.Context, &h.Databases, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputMove(
		ctx.GameID,
		chess.SerializeHistMove(result.Move),
		chess.SerializeGame(&result.State.Game),
		result.State.Touch,
	))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, h.Rdb, h.Rdb.GamesChan, bytes)
}

func (h *GameplayHandler) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	bytes, err := proto.Marshal(MakePbGameOutputChat(ctx.GameID, pbInput.Message))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, h.Rdb, h.Rdb.GamesChan, bytes)
}

func (h *GameplayHandler) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput) error {
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

	result, err := svc.AttemptGameUndo(ctx.Context, h.Databases.Rdb, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID,
		result,
	))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, h.Rdb, h.Rdb.GamesChan, bytes)
}
