package web

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
)

type GameSocketContext struct {
	*ServerState
	Context context.Context
	GameID  string
	Player  svc.PlayerState
}

func makeGameErr(ctx context.Context, gameID string, err error) []byte {
	var wsErr error
	switch err {
	case svc.ErrFinishedGame:
		wsErr = ErrWsFinishedGame
	case svc.ErrTurn:
		wsErr = ErrWsTurn
	case svc.ErrInvalidMove:
		wsErr = ErrWsInvalidMove
	case svc.ErrNoChessState:
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	default:
		wsErr = ErrWsFatal
	}
	slog.ErrorContext(ctx, "failed to error occurred while handling ws message", "err", err, "wsErr", wsErr)

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
	switch err {
	case svc.ErrNoChessState:
		wsErr = ErrWsInvalidGame
	default:
		wsErr = ErrWsFatal
	}
	slog.ErrorContext(ctx, "failed to error occurred in initializing gameplay websocket", "err", err, "wsErr", wsErr)
	return makeGameErr(ctx, gameID, wsErr)
}

func HandleGameWs(w http.ResponseWriter, r *http.Request, serverState *ServerState) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	player, err := svc.GetSession(ctx, serverState.Rdb, sessionID)
	if err != nil {
		writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
		return
	}
	chessState, err := svc.JoinGame(ctx, serverState.Rdb, gameID, player)
	if err != nil {
		writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
		return
	}

	if err := handleGameInit(gameID, player, chessState, func(b []byte) {
		writeConn(ctx, conn, b)
	}); err != nil {
		slog.ErrorContext(ctx, "failed to handle game init", "err", err)
		return
	}

	gameCtx := GameSocketContext{
		Context:     ctx,
		ServerState: serverState,
		GameID:      gameID,
		Player:      player,
	}

	writeChan := make(chan []byte)
	serverState.GamesCaster.Subscribe(gameID, writeChan)

	go func() {
		for b := range writeChan {
			writeConn(ctx, conn, b)
		}
	}()

	defer serverState.GamesCaster.Unsubscribe(gameID, writeChan)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "err", err)
			break
		}
		go handleGameMessage(gameCtx, msg, writeChan)
	}
}

func handleGameInit(gameID string, player svc.PlayerState, state *svc.ChessState, write func([]byte)) error {
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

func handleGameMessage(ctx GameSocketContext, msg []byte, writeChan chan []byte) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
		return
	}

	var err error
	if f := pbInput.GetForfeit(); f != nil {
		err = handleGameForfeit(ctx)
	} else if m := pbInput.GetMove(); m != nil {
		err = handleGameMove(ctx, m)
	} else if c := pbInput.GetChat(); c != nil {
		err = handleGameChat(ctx, c)
	} else if u := pbInput.GetUndo(); u != nil {
		err = handleGameUndo(ctx, u)
	} else {
		err = ErrWsMessageType
	}

	if err != nil {
		writeChan <- makeGameErr(ctx.Context, ctx.GameID, err)
	}
}

func handleGameForfeit(ctx GameSocketContext) error {
	if err := svc.ForfeitGame(ctx.Context, &ctx.Databases, ctx.GameID, ctx.Player); err != nil {
		return err
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(ctx.GameID))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, ctx.Rdb, ctx.Rdb.GamesChan, bytes)
}

func handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	result, err := svc.MakeGameMove(ctx.Context, &ctx.Databases, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
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
	return svc.BroadcastMessage(ctx.Context, ctx.Rdb, ctx.Rdb.GamesChan, bytes)
}

func handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	bytes, err := proto.Marshal(MakePbGameOutputChat(ctx.GameID, pbInput.Message))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx.Context, ctx.Rdb, ctx.Rdb.GamesChan, bytes)
}

func handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput) error {
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

	result, err := svc.AttemptGameUndo(ctx.Context, ctx.Databases.Rdb, ctx.GameID, ctx.Player, undoKind)
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
	return svc.BroadcastMessage(ctx.Context, ctx.Rdb, ctx.Rdb.GamesChan, bytes)
}
