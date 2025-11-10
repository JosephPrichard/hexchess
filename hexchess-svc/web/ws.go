package web

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/data"
	"hexchess-svc/pb"
	"log/slog"
	"net/http"
)

type WsHandler = func(w http.ResponseWriter, r *http.Request, ss ServerState)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type GameplayState struct {
	ServerState
	gameID string
	player data.PlayerState
}

func writeConn(b []byte, conn *websocket.Conn, ctx context.Context) {
	if b == nil {
		// a nil message is a "no-op", the caller does not need to check for errors
		return
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		slog.ErrorContext(ctx, "failed to write ws message", "err", err)
	}
}

func HandleGameplayWs(w http.ResponseWriter, r *http.Request, ss ServerState) {
	ctx := r.Context()
	gameID := r.URL.Query().Get("id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// writing the websocket error response and code is handled in the upgrade fn
		return
	}
	defer conn.Close()

	write := func(b []byte) {
		writeConn(b, conn, ctx)
	}
	writeInitErr := func(err error) {
		err = MapWsInitErr(err)
		slog.ErrorContext(ctx, "failed io initialize gameplay ws", "err", err, "wsErr", err)
		b := makeErrMsg(ctx, gameID, err)
		write(b)
	}

	player, _, err := GetSessionPlayer(ctx, ss.Rdb, r)
	if err != nil {
		writeInitErr(err)
		return
	}
	cs, err := data.JoinGame(ctx, ss.Rdb, gameID, player)
	if err != nil {
		writeInitErr(err)
		return
	}
	if err := handleInit(gameID, player, cs, write); err != nil {
		// callee is responsible for writing to the client, we just log in the caller
		slog.ErrorContext(ctx, "failed to handle game init", "err", err)
		return
	}

	state := GameplayState{ServerState: ss, gameID: gameID, player: player}

	writeChan := make(chan []byte)
	state.GamesCaster.Subscribe(state.gameID, writeChan)

	go func() {
		for b := range writeChan {
			write(b)
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
		handleMessage(ctx, state, msg, writeChan)
	}
}

func makeErrMsg(ctx context.Context, gameID string, err error) []byte {
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
			Message: err.Error(),
		}},
	})
	if err != nil {
		// this should never happen because the input is never unmarshall-able - if it does, we have no way to write back errors
		slog.ErrorContext(ctx, "failed to marshal err output msg", "err", err)
	}
	return b
}

func handleInit(gameID string, player data.PlayerState, state data.ChessState, write func([]byte)) error {
	pbState, err := data.MapPbChessState(state)
	if err != nil {
		return fmt.Errorf("failed to map pb state: %w", err)
	}
	outputs := []*pb.GameOutput{
		{
			GameId: gameID,
			Value: &pb.GameOutput_Init{
				Init: &pb.InitOutput{
					State: pbState,
					Self:  data.MapPbPlayer(&player),
				},
			},
		},
		{
			GameId: gameID,
			Value: &pb.GameOutput_Players{
				Players: &pb.PlayersOutput{
					WhitePlayer: data.MapPbPlayer(state.WhitePlayer),
					BlackPlayer: data.MapPbPlayer(state.BlackPlayer),
				},
			},
		},
	}
	for _, o := range outputs {
		b, err := proto.Marshal(o)
		if err != nil {
			return fmt.Errorf("failed to marshal game init output: %w", err)
		}
		write(b)
	}
	return nil
}

func handleMessage(ctx context.Context, gp GameplayState, msg []byte, writeChan chan []byte) {
	writeErr := func(err error) {
		err = MapWsEventErr(err)
		slog.ErrorContext(ctx, "error occurred while handling ws message", "err", err, "wsErr", err)
		b := makeErrMsg(ctx, gp.gameID, err)
		writeChan <- b
	}

	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		writeErr(err)
		return
	}

	var err error
	if f := pbInput.GetForfeit(); f != nil {
		err = handleForfeitMsg(ctx, gp)
	} else if m := pbInput.GetMove(); m != nil {
		err = handleMoveMsg(ctx, gp, m)
	} else if c := pbInput.GetChat(); c != nil {
		err = handleChatMsg(ctx, gp, c)
	} else {
		err = ErrWsMessageType
	}
	if err != nil {
		writeErr(err)
	}
}

func handleForfeitMsg(ctx context.Context, gp GameplayState) error {
	if err := data.ForfeitGame(ctx, gp.Stores, gp.gameID, gp.player); err != nil {
		return err
	}
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: gp.gameID,
		Value:  &pb.GameOutput_Forfeit{Forfeit: &pb.ForfeitOutput{}},
	})
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, gp.Rdb, gp.Rdb.GamesChan, b)
}

func handleMoveMsg(ctx context.Context, gp GameplayState, pbInput *pb.MoveInput) error {
	mr, err := data.MakeGameMove(ctx, gp.Stores, gp.gameID, gp.player, data.MapPieceMove(pbInput.Move))
	if err != nil {
		return err
	}
	pbGame, err := data.MapPbGame(mr.Room.Game)
	if err != nil {
		return err
	}
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: gp.gameID,
		Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
			PieceMove: data.MapPbPieceMove(mr.Move),
			Game:      pbGame,
		}},
	})
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, gp.Rdb, gp.Rdb.GamesChan, b)
}

func handleChatMsg(ctx context.Context, gp GameplayState, pbInput *pb.ChatInput) error {
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: gp.gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
			Message: pbInput.Message,
		}},
	})
	if err != nil {
		return err
	}
	return data.BroadcastMessage(ctx, gp.Rdb, gp.Rdb.GamesChan, b)
}
