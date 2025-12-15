package web

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/svc"
	"log/slog"
	"net/http"
)

type GameSocketState struct {
	ServerState
	gameID string
	player svc.PlayerState
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
		slog.ErrorContext(ctx, "failed to marshal err output msg", "err", err)
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

func HandleGameWs(w http.ResponseWriter, r *http.Request, serverState ServerState) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var sessPlayer *svc.PlayerState

	player, err := svc.GetSession(ctx, serverState.Rdb, sessionID)
	if err != nil {
		switch err {
		case svc.ErrSessionNotFound:
			sessPlayer = nil
		default:
			writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
			return
		}
	} else {
		sessPlayer = &player
	}
	chessState, err := svc.JoinGame(ctx, serverState.Rdb, gameID, sessPlayer)
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

	state := GameSocketState{
		ServerState: serverState,
		gameID:      gameID,
		player:      player,
	}

	writeChan := make(chan []byte)
	state.GamesCaster.Subscribe(state.gameID, writeChan)

	go func() {
		for b := range writeChan {
			writeConn(ctx, conn, b)
		}
	}()

	defer state.GamesCaster.Unsubscribe(state.gameID, writeChan)
	for {
		_, msg, err := conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			break
		}
		if err != nil {
			slog.ErrorContext(ctx, "failed to read ws message", "err", err)
			break
		}
		handleGameMessage(ctx, state, msg, writeChan)
	}
}

func handleGameInit(gameID string, player svc.PlayerState, state *svc.ChessState, write func([]byte)) error {
	for i, o := range []*pb.GameOutput{
		MakePbGameOutputInit(
			gameID,
			svc.SerializeChessState(state),
			svc.SerializePlayer(&player),
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

func handleGameMessage(ctx context.Context, state GameSocketState, msg []byte, writeChan chan []byte) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		writeChan <- makeGameErr(ctx, state.gameID, err)
		return
	}

	var err error
	if f := pbInput.GetForfeit(); f != nil {
		err = handleGameForfeit(ctx, state)
	} else if m := pbInput.GetMove(); m != nil {
		err = handleGameMove(ctx, state, m)
	} else if c := pbInput.GetChat(); c != nil {
		err = handleGameChat(ctx, state, c)
	} else if u := pbInput.GetUndo(); u != nil {
		err = handleGameUndo(ctx, state, u)
	} else {
		err = ErrWsMessageType
	}

	if err != nil {
		writeChan <- makeGameErr(ctx, state.gameID, err)
	}
}

func handleGameForfeit(ctx context.Context, state GameSocketState) error {
	if err := svc.ForfeitGame(ctx, &state.Databases, state.gameID, state.player); err != nil {
		return err
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(state.gameID))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}

func handleGameMove(ctx context.Context, state GameSocketState, pbInput *pb.MoveInput) error {
	result, err := svc.MakeGameMove(ctx, &state.Databases, state.gameID, state.player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputMove(
		state.gameID,
		chess.SerializeHistMove(result.Move),
		chess.SerializeGame(&result.State.Game),
		result.State.Touch,
	))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}

func handleGameChat(ctx context.Context, state GameSocketState, pbInput *pb.ChatInput) error {
	bytes, err := proto.Marshal(MakePbGameOutputChat(state.gameID, pbInput.Message))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}

func handleGameUndo(ctx context.Context, state GameSocketState, pbInput *pb.UndoInput) error {
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

	result, err := svc.AttemptGameUndo(ctx, state.Databases.Rdb, state.gameID, state.player, undoKind)
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputUndo(
		state.gameID,
		pbInput.Kind,
		state.player.ID,
		result,
	))
	if err != nil {
		return err
	}
	return svc.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}
