package web

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/pb"
	"log/slog"
	"net/http"
)

type GameSocketState struct {
	ServerState
	gameID string
	player data.PlayerState
}

func makeGameErr(ctx context.Context, gameID string, err error) []byte {
	var wsErr error
	switch err {
	case data.ErrFinishedGame:
		wsErr = ErrWsFinishedGame
	case data.ErrTurn:
		wsErr = ErrWsTurn
	case data.ErrInvalidMove:
		wsErr = ErrWsInvalidMove
	case data.ErrNoChessState:
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	default:
		wsErr = ErrWsFatal
	}
	slog.ErrorContext(ctx, "error occurred while handling ws message", "err", err, "wsErr", wsErr)

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
	case data.ErrNoChessState:
		wsErr = ErrWsInvalidGame
	default:
		wsErr = ErrWsFatal
	}
	slog.ErrorContext(ctx, "error occurred in initializing gameplay websocket", "err", err, "wsErr", wsErr)
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

	var sessPlayer *data.PlayerState

	player, err := data.GetSession(ctx, serverState.Rdb, sessionID)
	if err != nil {
		switch err {
		case data.ErrSessionNotFound:
			sessPlayer = nil
		default:
			writeConn(ctx, conn, makeGameInitErr(ctx, gameID, err))
			return
		}
	} else {
		sessPlayer = &player
	}
	chessState, err := data.JoinGame(ctx, serverState.Rdb, gameID, sessPlayer)
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

func handleGameInit(gameID string, player data.PlayerState, state data.ChessState, write func([]byte)) error {
	for i, o := range []*pb.GameOutput{
		MakePbGameOutputInit(
			gameID,
			data.SerializeChessState(state),
			data.SerializePlayer(&player),
		),
		MakePbGameOutputPlayers(
			gameID,
			data.SerializePlayer(state.WhitePlayer),
			data.SerializePlayer(state.BlackPlayer),
		),
	} {
		b, err := proto.Marshal(o)
		if err != nil {
			return fmt.Errorf("failed to marshal game init output %d: %w", i, err)
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
	} else {
		err = ErrWsMessageType
	}

	if err != nil {
		writeChan <- makeGameErr(ctx, state.gameID, err)
	}
}

func handleGameForfeit(ctx context.Context, state GameSocketState) error {
	if err := data.ForfeitGame(ctx, &state.Databases, state.gameID, state.player); err != nil {
		return err
	}
	bytes, err := proto.Marshal(MakePbGameOutputForfeit(state.gameID))
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}

func handleGameMove(ctx context.Context, state GameSocketState, pbInput *pb.MoveInput) error {
	result, err := data.MakeGameMove(ctx, &state.Databases, state.gameID, state.player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(MakePbGameOutputMove(
		state.gameID,
		chess.SerializeHistMove(result.Move),
		chess.SerializeGame(result.Room.Game),
		result.Room.Touch,
	))
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}

func handleGameChat(ctx context.Context, state GameSocketState, pbInput *pb.ChatInput) error {
	bytes, err := proto.Marshal(MakePbGameOutputChat(state.gameID, pbInput.Message))
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, state.Rdb, state.Rdb.GamesChan, bytes)
}
