package web

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/chess"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/assertutil"
	svc "hexchess-svc/services"

	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestHandleGameplayWs is a high-level black box testing that checks the broadcast and websocket output for every input case
// Database assertions run after message assertions and can assume that inbound websocket messages are valid
func TestHandleGameplayWs(t *testing.T) {
	gameID := TestGameID1

	wantInit := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
			State: &pb.ChessState{}, // ignoring state, we care about message count/type here.
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
	wantBgInit := &pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_BgInit{BgInit: &pb.BgInitOutput{}}}
	wantForfeit := &pb.GameOutput_Forfeit{Forfeit: &pb.ForfeitOutput{
		ReplayId: 1,
		EndState: &pb.EndState{Value: &pb.EndState_FinishState{
			FinishState: &pb.FinishState{
				WinEloDiff:  15,
				LoseEloDiff: -15,
				Cause:       "FORFEIT",
				Result:      "BLACK_WINS",
			}},
		},
	}}
	wantValidMove := &pb.GameOutput_Move{Move: &pb.MoveOutput{
		UpdatedAt: itest.TimeNow.Format(time.RFC3339),
		Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, CollFile: true, CollRank: true},
	}}
	wantChat := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
			Message: "Hello World",
			Player:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			SentAt:  itest.TimeNow.Format(time.RFC3339),
		}},
	}

	wantMsgs := []*pb.GameOutput{wantInit, wantPlayers, wantBgInit}
	wantBrdcasts := []any{wantPlayers, wantBgInit}

	for _, test := range []struct {
		name         string
		inputMsgs    []*pb.GameInput
		wantMsgs     []*pb.GameOutput
		wantBrdcasts []any
	}{
		{
			name: "move input (invalid)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{}}}},
			},
			wantMsgs: []*pb.GameOutput{
				{
					GameId: gameID,
					Value:  &pb.GameOutput_Error{Error: &pb.ErrorOutput{Message: ErrWsInvalidMove.Error()}},
				},
			},
		},
		{
			name: "move input (valid)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: chess.PbMoveStr("b1", "b2")}}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: wantValidMove},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: wantValidMove},
			},
		},
		{
			name: "chat input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}},
			},
			wantMsgs:     []*pb.GameOutput{wantChat},
			wantBrdcasts: []any{wantChat},
		},
		{
			name: "forfeit input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Forfeit{}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: wantForfeit},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: wantForfeit},
			},
		},
		{
			name: "undo input (success)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "REJECT"}}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "REJECT", UndoId: 1}}},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "REJECT", UndoId: 1}}},
			},
		},
		{
			name: "undo input (fail)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "ACCEPT"}}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{Message: ErrWsUndoAction.Error()}}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			wantMsgs := slices.Concat(wantMsgs, test.wantMsgs)
			wantBrdcasts := slices.Concat(wantBrdcasts, test.wantBrdcasts)

			state := svc.SetupStateTest(t, itest.UseTxn, itest.WithPostgres, itest.WithRedis, itest.WithAws)
			defer state.Close()

			state.EntropySource = &ext.StableSource{Time: itest.TimeNow}
			state.LocalBroadcasters = svc.MakeBroadcaster()

			createTestSessions(t, state)
			createTestChessStates(t, state)

			ts := httptest.NewServer(MakeRoot(Setup{State: state}))
			defer ts.Close()

			// (start, subcribe, and read broadcasts)
			<-state.LocalBroadcasters.ListenGameMessages(state.Redis)
			subChan := make(chan []byte, len(wantBrdcasts))
			state.LocalBroadcasters.GamesCaster.Subscribe(gameID, subChan)

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			// when
			url := strings.Replace(fmt.Sprintf("%s/api/ws/game?gameId=%s&sessionId=%s", ts.URL, gameID, TestSessionID1), "http", "ws", 1)
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, http.Header{})
			require.NoError(t, err)
			defer conn.Close()

			for _, input := range test.inputMsgs {
				b, err := proto.Marshal(input)
				require.NoError(t, err)
				require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, b))
			}

			// then
			msgs := make([]*pb.GameOutput, len(wantMsgs))
			for i := range wantMsgs {
				_, b, err := conn.ReadMessage()
				require.NoError(t, err)

				output := &pb.GameOutput{}
				require.NoError(t, proto.Unmarshal(b, output))

				msgs[i] = output
			}

			brdcasts := readBroadcasts(ctx, len(wantBrdcasts), subChan)

			cmpOpts := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.InitOutput{}, "state"),
				protocmp.IgnoreFields(&pb.MoveOutput{}, "game"),
				protocmp.IgnoreFields(&pb.UndoOutput{}, "game"),
				protocmp.IgnoreFields(&pb.HistMove{}, "madeOn"),
				protocmp.IgnoreFields(&pb.ForfeitOutput{}, "replay_id"),
			}

			slices.SortFunc(msgs, func(a *pb.GameOutput, b *pb.GameOutput) int {
				return getMsgSortOrd(a) - getMsgSortOrd(b)
			})
			slices.SortFunc(brdcasts, func(a any, b any) int {
				return getBrdcastSortOrd(a) - getBrdcastSortOrd(b)
			})
			assertutil.Equal(t, wantMsgs, msgs, cmpOpts...)
			assertutil.Equal(t, wantBrdcasts, brdcasts, cmpOpts...)
		})
	}
}

func readBroadcasts(ctx context.Context, wantBrdcasts int, subChan chan []byte) []any {
	var brdcasts []any
ReadBrdcasts:
	for range wantBrdcasts {
		select {
		case b, ok := <-subChan:
			if !ok {
				break ReadBrdcasts
			}
			var pbGame pb.GameOutput
			if err := proto.Unmarshal(b, &pbGame); err != nil {
				brdcasts = append(brdcasts, err)
			} else {
				brdcasts = append(brdcasts, &pbGame)
			}
		case <-ctx.Done():
			break ReadBrdcasts
		}
	}
	return brdcasts
}

// getMsgSortOrd and getBrdcastSortOrd create deterministic orderings of messages that are used in the assertions of websocket tests

func getMsgSortOrd(o *pb.GameOutput) int {
	switch o.Value.(type) {
	case *pb.GameOutput_Init:
		return 1
	case *pb.GameOutput_Players:
		return 2
	case *pb.GameOutput_BgInit:
		return 3
	case *pb.GameOutput_Move:
		return 4
	case *pb.GameOutput_Chat:
		return 5
	case *pb.GameOutput_Forfeit:
		return 6
	case *pb.GameOutput_Undo:
		return 7
	case *pb.GameOutput_Error:
		return 8
	default:
		panic(fmt.Sprintf("unexpected output type: %T", o.Value))
	}
}

func getBrdcastSortOrd(b any) int {
	switch o := b.(type) {
	case *pb.GameOutput:
		return getMsgSortOrd(o)
	case error:
		return 1
	default:
		panic(fmt.Sprintf("unexpected output type: %T", b))
	}
}
