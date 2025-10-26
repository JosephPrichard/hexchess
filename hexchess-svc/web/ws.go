package web

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"hexchess-svc/pb"
	"log/slog"
	"net/http"
)

type WsHandler = func(w http.ResponseWriter, r *http.Request, ss ServerState) error

func makeWsHandler(state ServerState, h WsHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := uuid.NewString()
		r = r.WithContext(context.WithValue(r.Context(), lib.TK, trace))

		slog.InfoContext(r.Context(), "ws received", "method", r.Method, "url", r.URL)

		if err := h(w, r, state); err != nil {
			slog.ErrorContext(r.Context(), "ws failed", "method", r.Method, "url", r.URL, "error", err)

			status, m := HttpStatusFromErr(err)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(m))
		}
	})
}

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

func HandleGameplayWs(w http.ResponseWriter, r *http.Request, ss ServerState) error {
	ctx := r.Context()

	gameID := r.URL.Query().Get("id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer slog.InfoContext(ctx, "closed ws connection")

	player, _, err := GetSessionPlayer(ctx, ss.Rdb, r)
	if err != nil {
		return err
	}
	gp := GameplayState{ServerState: ss, gameID: gameID, player: player}

	state, err := JoinGame(ctx, gp.Stores, gp.gameID, player)
	if err != nil {
		return err
	}
	if err := handleGameplayInit(gp, conn, state); err != nil {
		slog.ErrorContext(ctx, "failed to handle gameplay init", "err", err)
		return ErrWsFatal
	}

	writeChan := make(chan []byte)
	gp.GamesCaster.Subscribe(gp.gameID, writeChan)
	defer gp.GamesCaster.Unsubscribe(gp.gameID, writeChan)

	go func() {
		for b := range writeChan {
			if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
				slog.ErrorContext(ctx, "failed to write ws message", "err", err)
			}
		}
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			break
		}
		if err != nil {
			slog.ErrorContext(ctx, "failed to read ws message", "err", err)
			break
		}
		handleMsg(ctx, gp, msg, writeChan)
	}
	return nil
}

func handleGameplayInit(gp GameplayState, conn *websocket.Conn, state data.ChessState) error {
	pbState, err := data.MapPbChessState(state)
	if err != nil {
		return fmt.Errorf("failed to map pb state: %w", err)
	}
	output1 := &pb.GameOutput{
		GameId: gp.gameID,
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{
				State: pbState,
				Self:  data.MapPbPlayer(&gp.player),
			},
		},
	}
	output2 := &pb.GameOutput{
		GameId: gp.gameID,
		Value: &pb.GameOutput_Players{
			Players: &pb.PlayersOutput{
				WhitePlayer: data.MapPbPlayer(state.WhitePlayer),
				BlackPlayer: data.MapPbPlayer(state.BlackPlayer),
			},
		},
	}
	for _, o := range []*pb.GameOutput{
		output1,
		output2,
	} {
		b, err := proto.Marshal(o)
		if err != nil {
			return fmt.Errorf("failed to marshal output msg: %w", err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
			return fmt.Errorf("failed to write ws msg: %w", err)
		}
	}
	return nil
}

func handleMsg(ctx context.Context, gp GameplayState, msg []byte, writeChan chan []byte) {
	sendErr := func(err error) {
		wsErr := mapWsErr(err)
		slog.ErrorContext(ctx, "error occurred while handling ws message", "err", err, "wsErr", wsErr)
		b, err := proto.Marshal(&pb.GameOutput{
			GameId: gp.gameID,
			Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
				Message: wsErr.Error(),
			}},
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal err output msg", "err", err)
			return
		}
		writeChan <- b
	}

	var pbInput pb.GameInput
	if err := proto.Unmarshal(msg, &pbInput); err != nil {
		sendErr(err)
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
		sendErr(err)
	}
}

func handleForfeitMsg(ctx context.Context, gp GameplayState) error {
	if err := ForfeitGame(ctx, gp.Stores, gp.gameID, gp.player); err != nil {
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
	mr, err := MakeGameMove(ctx, gp.Stores, gp.gameID, gp.player, data.MapPieceMove(pbInput.Move))
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
