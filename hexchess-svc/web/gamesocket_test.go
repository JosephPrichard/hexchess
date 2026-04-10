package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"hexchess-svc/chess"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/service"
	"hexchess-svc/util/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestHandleGameplayWs is a high-level black box testing that checks the broadcast and websocket output for every input case
// Database assertions run after message assertions and can assume that inbound websocket messages are valid
func TestHandleGameplayWs(t *testing.T) {
	t.Parallel()

	gameID := TestGameID1

	wantInit := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
			State: &pb.ChessState{}, // ignoring services, we care about message count/type here.
			Self:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
		}},
	}
	wantPlayers := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
			WhitePlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			BlackPlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
		}},
	}
	wantValidMove := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
			UpdatedAt: itest.TimeNow.Format(time.RFC3339),
			Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, Notation: "Pb2"},
		}},
	}
	wantChat := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Message: "Hello World",
			Player:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			SentAt:  itest.TimeNow.Format(time.RFC3339),
		}},
	}
	wantForfeit := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: pb.EndKind_FINISHED,
			},
		},
	}
	wantUndo := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Undo{
			Undo: &pb.UndoOutput{Kind: "REJECT", UndoId: 1},
		},
	}

	tests := []struct {
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
					GameId: gameID,
					Value: &pb.GameOutput_Error{
						Error: &pb.ErrorOutput{Message: ErrWsInvalidMove.Error()},
					},
				},
			},
			wantBrdcasts: []any{
				wantPlayers,
			},
		},
		{
			name: "MoveValid",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: chess.PbMoveStr("b1", "b2")}},
			},
			wantMsgs: []*pb.GameOutput{
				wantValidMove,
				wantPlayers,
				wantInit,
			},
			wantBrdcasts: []any{
				wantValidMove,
				wantPlayers,
			},
		},
		{
			name: "Chat",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}},
			},
			wantMsgs: []*pb.GameOutput{
				wantChat,
				wantPlayers,
				wantInit,
			},
			wantBrdcasts: []any{
				wantChat,
				wantPlayers,
			},
		},
		{
			name: "Forfeit",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Forfeit{Forfeit: &pb.ForfeitInput{}},
			},
			wantMsgs: []*pb.GameOutput{
				wantForfeit,
				wantPlayers,
				wantInit,
			},
			wantBrdcasts: []any{
				wantForfeit,
				wantPlayers,
			},
		},
		{
			name: "UndoInput",
			inputMsg: &pb.GameInput{
				Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "REJECT"}},
			},
			wantMsgs: []*pb.GameOutput{
				wantUndo,
				wantPlayers,
				wantInit,
			},
			wantBrdcasts: []any{
				wantUndo,
				wantPlayers,
			},
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
					GameId: gameID,
					Value: &pb.GameOutput_Error{
						Error: &pb.ErrorOutput{Message: ErrWsUndoAction.Error()},
					},
				},
			},
			wantBrdcasts: []any{
				wantPlayers,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsgs := tt.wantMsgs
			wantBrdcasts := tt.wantBrdcasts

			mocks := svc.ServiceMocks{Entropy: &svc.StableEntropySource{Time: itest.TimeNow}}
			services, testinfra := svc.SetupServicesTest(t, mocks, itest.RWPostgres, itest.Redis)
			defer services.Close()

			broadcasters := svc.MakeLocalBroadcasters()

			createTestSessions(t, services)
			createTestChessStates(t, services)

			testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, EntropySource: mocks.Entropy, Broadcasers: broadcasters}))
			defer testServer.Close()

			// (start, subscribe, and read broadcasts)
			<-broadcasters.ListenGameMessages(testinfra.Redis)
			subChan := make(chan []byte, len(wantBrdcasts))
			broadcasters.GamesCaster.Subscribe(gameID, subChan)

			url := strings.Replace(fmt.Sprintf("%s/api/ws/game?gameId=%s&sessionId=%s", testServer.URL, gameID, TestSessionID1), "http", "ws", 1)
			conn, _, err := websocket.DefaultDialer.DialContext(t.Context(), url, http.Header{})
			require.NoError(t, err)
			defer conn.Close()

			bytes, err := proto.Marshal(tt.inputMsg)
			require.NoError(t, err)
			require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, bytes))

			msgs := make([]*pb.GameOutput, len(wantMsgs))
			for i := range wantMsgs {
				_, bytes, err := conn.ReadMessage()
				require.NoError(t, err)

				output := &pb.GameOutput{}
				require.NoError(t, proto.Unmarshal(bytes, output))

				msgs[i] = output
			}

			brdcasts := readBroadcasts(t.Context(), len(wantBrdcasts), subChan)

			cmpOpts := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.InitOutput{}, "state"),
				protocmp.IgnoreFields(&pb.MoveOutput{}, "game"),
				protocmp.IgnoreFields(&pb.UndoOutput{}, "game"),
				protocmp.IgnoreFields(&pb.HistMove{}, "white_timer_ms", "black_timer_ms"),
			}

			slices.SortFunc(msgs, func(a *pb.GameOutput, b *pb.GameOutput) int {
				return getMessageSortOrd(b) - getMessageSortOrd(a)
			})
			slices.SortFunc(brdcasts, func(a any, b any) int {
				return getBroadcastSortOrd(b) - getBroadcastSortOrd(a)
			})
			testutil.Equal(t, wantMsgs, msgs, cmpOpts...)
			testutil.Equal(t, wantBrdcasts, brdcasts, cmpOpts...)
		})
	}
}

func readBroadcasts(ctx context.Context, wantBrdcasts int, subChan chan []byte) []any {
	var brdcasts []any
	for range wantBrdcasts {
		var bytes []byte

		select {
		case <-ctx.Done():
			return brdcasts
		case bytes = <-subChan:
		}

		var pbGame pb.GameOutput
		if err := proto.Unmarshal(bytes, &pbGame); err != nil {
			brdcasts = append(brdcasts, err)
		} else {
			brdcasts = append(brdcasts, &pbGame)
		}
	}
	return brdcasts
}

// getMsgSortOrd and getBrdcastSortOrd create deterministic orderings of messages that are used in the assertions of websocket tests

func getMessageSortOrd(o *pb.GameOutput) int {
	oneof := o.ProtoReflect().Descriptor().Oneofs().ByName("value")
	whichField := o.ProtoReflect().WhichOneof(oneof)
	if whichField == nil {
		return 0
	}
	return int(whichField.Number())
}

func getBroadcastSortOrd(b any) int {
	switch o := b.(type) {
	case *pb.GameOutput:
		return getMessageSortOrd(o)
	default:
		return 0
	}
}
