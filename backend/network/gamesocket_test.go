package network

import (
	stlcmp "cmp"
	"context"
	"fmt"
	"hexchess-svc/chess"

	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/utils/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

var (
	gameID = TestGameID1

	wantInit = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
			State: &pb.ChessState{},
			Self:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
		}},
	}
	wantPlayers = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
			WhitePlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			BlackPlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
		}},
	}
	wantValidMove = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
			UpdatedAt: itest.TimeNow.Format(time.RFC3339),
			Move:      &pb.HistMove{Piece: uint32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, Notation: "Pb2"},
		}},
	}
	wantChat = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Message: "Hello World",
			Player:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			SentAt:  itest.TimeNow.Format(time.RFC3339),
		}},
	}
	wantForfeit = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: pb.EndKind_FINISHED,
			},
		},
	}
	wantUndo = &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Undo{
			Undo: &pb.UndoOutput{Kind: "REJECT", UndoId: 1},
		},
	}

	gameOutputTests = []struct {
		name         string
		inputMsg     *pb.GameInput
		wantMsgs     []*pb.GameOutput
		wantBrdcasts []any
	}{
		{
			name: "MoveInvalid",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Move{
					Move: &pb.MoveInput{Move: &pb.Move{}},
				},
			},
			wantMsgs: []*pb.GameOutput{
				wantPlayers,
				wantInit,
				{
					GameId: gameID.String(),
					Value: &pb.GameOutput_Error{
						Error: &pb.ErrorOutput{Message: ErrWsInvalidMove.Error()},
					},
				},
			},
			wantBrdcasts: []any{wantPlayers},
		},
		{
			name: "MoveValid",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: chess.PbMoveStr("b1", "b2")}},
			},
			wantMsgs:     []*pb.GameOutput{wantValidMove, wantPlayers, wantInit},
			wantBrdcasts: []any{wantValidMove, wantPlayers},
		},
		{
			name: "Chat",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}},
			},
			wantMsgs:     []*pb.GameOutput{wantChat, wantPlayers, wantInit},
			wantBrdcasts: []any{wantChat, wantPlayers},
		},
		{
			name: "Forfeit",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Forfeit{Forfeit: &pb.ForfeitInput{}},
			},
			wantMsgs:     []*pb.GameOutput{wantForfeit, wantPlayers, wantInit},
			wantBrdcasts: []any{wantForfeit, wantPlayers},
		},
		{
			name: "UndoInput",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "REJECT"}},
			},
			wantMsgs:     []*pb.GameOutput{wantUndo, wantPlayers, wantInit},
			wantBrdcasts: []any{wantUndo, wantPlayers},
		},
		{
			name: "UndoInputError",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "ACCEPT"}},
			},
			wantMsgs: []*pb.GameOutput{
				wantPlayers,
				wantInit,
				{
					GameId: gameID.String(),
					Value: &pb.GameOutput_Error{
						Error: &pb.ErrorOutput{Message: ErrWsUndoAction.Error()},
					},
				},
			},
			wantBrdcasts: []any{wantPlayers},
		},
	}
)

func TestHandleGameplayWs(t *testing.T) {
	t.Parallel()

	for _, tt := range gameOutputTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			wantMsgs := tt.wantMsgs
			wantBrdcasts := tt.wantBrdcasts

			websocketTest := setupWebsocketTest(t)
			defer websocketTest.Shutdown()

			subChan := make(chan []byte, len(wantBrdcasts))
			websocketTest.broadcasters.Games.Subscribe(gameID, subChan)

			params := url.Values{}
			params.Set("gameId", gameID.String())
			params.Set("sessionId", TestSessionID1)

			conn, _, err := websocket.DefaultDialer.DialContext(ctx,
				fmt.Sprintf("%s/api/ws/game?%s", websocketTest.getWsURL(), params.Encode()),
				http.Header{})
			require.NoError(t, err)
			defer conn.Close()

			inputBytes, err := proto.Marshal(tt.inputMsg)
			require.NoError(t, err)

			err = conn.WriteMessage(websocket.BinaryMessage, inputBytes)
			require.NoError(t, err)

			msgs := readGameOutputMsgs(t, conn, len(wantMsgs))
			brdcasts := readGameOutputBroadcasts(ctx, len(wantBrdcasts), subChan)

			testutil.Equal(t, wantMsgs, msgs, gameOutputAssertionCmpOpts...)
			testutil.Equal(t, wantBrdcasts, brdcasts, gameOutputAssertionCmpOpts...)
		})
	}
}

var gameOutputAssertionCmpOpts = []cmp.Option{
	protocmp.Transform(),
	protocmp.IgnoreFields(&pb.InitOutput{}, "state"),
	protocmp.IgnoreFields(&pb.MoveOutput{}, "game"),
	protocmp.IgnoreFields(&pb.UndoOutput{}, "game"),
	protocmp.IgnoreFields(&pb.ChatMessage{}, "sent_at"),
	protocmp.IgnoreFields(&pb.MoveOutput{}, "updated_at"),
	protocmp.IgnoreFields(&pb.HistMove{}, "white_timer_ms", "black_timer_ms"),
}

func readGameOutputMsgs(t *testing.T, conn *websocket.Conn, wantMsgs int) []*pb.GameOutput {
	msgs := make([]*pb.GameOutput, 0, wantMsgs)
	for range wantMsgs {
		_, outputBytes, err := conn.ReadMessage()
		require.NoError(t, err)

		output := &pb.GameOutput{}
		require.NoError(t, proto.Unmarshal(outputBytes, output))

		msgs = append(msgs, output)
	}

	slices.SortFunc(msgs, func(a *pb.GameOutput, b *pb.GameOutput) int {
		return stlcmp.Compare(getMessageSortOrd(b), getMessageSortOrd(a))
	})
	return msgs
}

func readGameOutputBroadcasts(ctx context.Context, wantBrdcasts int, subChan chan []byte) []any {
	var broadcasts []any
	for range wantBrdcasts {
		var bytes []byte

		select {
		case <-ctx.Done():
			return broadcasts
		case bytes = <-subChan:
		}

		var pbGame pb.GameOutput
		if err := proto.Unmarshal(bytes, &pbGame); err != nil {
			broadcasts = append(broadcasts, err)
		} else {
			broadcasts = append(broadcasts, &pbGame)
		}
	}

	getBroadcastSortOrd := func(b any) int {
		switch o := b.(type) {
		case *pb.GameOutput:
			return getMessageSortOrd(o)
		default:
			return 0
		}
	}
	slices.SortFunc(broadcasts, func(a any, b any) int {
		return stlcmp.Compare(getBroadcastSortOrd(b), getBroadcastSortOrd(a))
	})

	return broadcasts
}

func getMessageSortOrd(o *pb.GameOutput) int {
	oneof := o.ProtoReflect().Descriptor().Oneofs().ByName("value")
	whichField := o.ProtoReflect().WhichOneof(oneof)
	if whichField == nil {
		return 0
	}
	return int(whichField.Number())
}
