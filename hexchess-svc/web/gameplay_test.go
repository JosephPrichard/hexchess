package web

import (
	// "context"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/assertutil"
	// "hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func readBroadcasted(outputsChan chan []any, subChan chan []byte, count int) {
	var outputs []any
	for range count {
		b, ok := <-subChan
		if !ok {
			break
		}
		var pbGame pb.GameOutput
		if err := proto.Unmarshal(b, &pbGame); err != nil {
			outputs = append(outputs, err)
		} else {
			outputs = append(outputs, &pbGame)
		}
	}
	outputsChan <- outputs
}

// TestHandleGameplayWs is a high level black box test that
// checks the broadcast and websocket output for every message case, and asserts in the database when needed
// database assertions run after message assertions and can assume that inbound websocket messages are valid
func TestHandleGameplayWs(t *testing.T) {
	wantState := TestStates[0].DeepCopy()
	wantState.WhitePlayer = svc.MakePlayer(1, "user1", "us")

	cmnWantMsgs := []*pb.GameOutput{
		{
			GameId: TestGameID1,
			Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
				Self:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
				State: svc.SerializeChessState(&wantState),
			}},
		},
		{
			GameId: TestGameID1,
			Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
				BlackPlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
				WhitePlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			}},
		},
	}

	// ctx := context.WithValue(t.Context(), logutil.Trace, "test-handle-gameplay-ws")

	for _, test := range []struct {
		name            string
		inputMsgs       []*pb.GameInput
		wantMsgs        []*pb.GameOutput
		wantBrdcasts    []any
		assertDatabases func(*testing.T, *db.Databases, []*pb.GameOutput)
	}{
		// {
		// 	name: "move input (invalid)",
		// 	inputMsgs: []*pb.GameInput{
		// 		{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{}}}},
		// 	},
		// 	wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
		// 		{
		// 			GameId: TestGameID1,
		// 			Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
		// 				Message: ErrWsInvalidMove.Error(),
		// 			}},
		// 		},
		// 	}),
		// },
		{
			name: "move input (valid)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: chess.PbMoveStr("b1", "b2")}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{
					GameId: TestGameID1,
					Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
						Message: ErrWsTurn.Error(),
					}},
				},
			}),
		},
		// {
		// 	name: "chat input",
		// 	inputMsgs: []*pb.GameInput{
		// 		{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}},
		// 	},
		// 	wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
		// 		{
		// 			GameId: TestGameID1,
		// 			Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
		// 				Message: "Hello World",
		// 			}},
		// 		},
		// 	}),
		// 	wantBrdcasts: []any{
		// 		&pb.GameOutput{
		// 			GameId: TestGameID1,
		// 			Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
		// 				Message: "Hello World",
		// 			}},
		// 		},
		// 	},
		// },
		// {
		// 	name: "forfeit",
		// 	inputMsgs: []*pb.GameInput{
		// 		{Value: &pb.GameInput_Forfeit{}},
		// 	},
		// 	wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
		// 		{
		// 			GameId: TestGameID1,
		// 			Value:  &pb.GameOutput_Forfeit{},
		// 		},
		// 	}),
		// 	wantBrdcasts: []any{
		// 		&pb.GameOutput{
		// 			GameId: TestGameID1,
		// 			Value:  &pb.GameOutput_Forfeit{},
		// 		},
		// 	},
		// 	assertDatabases: func(t *testing.T, databases *db.Databases, msgOutputs []*pb.GameOutput) {
		// 		replayID := msgOutputs[2].GetForfeit().ReplayId // we can assume this is valid if the test reaches this point

		// 		actualState, err := svc.GetChessState(ctx, databases.Rdb, wantState.ID)
		// 		require.NoError(t, err)
		// 		assert.True(t, actualState.IsEnded)

		// 		replay, err := databases.Pdb.Query.SelectReplayRowByID(context.Background(), replayID)
		// 		require.NoError(t, err)
		// 		assert.Equal(t, replay.Cause, db.CauseEnum("FORFEIT"))
		// 	},
		// },
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			databases, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			createTestSessions(t, databases.Rdb)
			createTestChessStates(t, databases.Rdb)

			state := State{Databases: databases, Broadcasters: svc.MakeBroadcaster()}
			ts := httptest.NewServer(HandleRoot(state))
			defer ts.Close()

			// (start, subcribe, and read broadcasts)
			<-state.Broadcasters.ListenGameMessages(databases.Rdb)
			subChan := make(chan []byte)
			state.Broadcasters.GamesCaster.Subscribe(TestGameID1, subChan)
			brdCastChan := make(chan []any)
			go readBroadcasted(brdCastChan, subChan, len(test.wantBrdcasts))

			// when
			url := strings.Replace(fmt.Sprintf("%s/api/ws/game?gameId=%s&sessionId=%s", ts.URL, TestGameID1, TestSessionID1), "http", "ws", 1)
			conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
			if err != nil {
				t.Fatalf("dial websocket: %v", err)
			}
			defer conn.Close()

			for _, input := range test.inputMsgs {
				b, err := proto.Marshal(input)
				if err != nil {
					t.Fatalf("marshal input: %v", err)
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
					t.Fatalf("write message: %v", err)
				}
			}

			// then
			msgOutputs := make([]*pb.GameOutput, len(test.wantMsgs))
			for i := range test.wantMsgs {
				_, b, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("read ws message: %v", err)
				}
				msgOutputs[i] = &pb.GameOutput{}
				if err := proto.Unmarshal(b, msgOutputs[i]); err != nil {
					t.Fatalf("marshal input: %v", err)
				}
			}
			brdCastOutputs := <-brdCastChan

			assertutil.AssertEqualIgnoring(t, test.wantMsgs, msgOutputs, protocmp.Transform(), protocmp.IgnoreFields(&pb.ChessState{}, "touch"), protocmp.IgnoreFields(&pb.ForfeitOutput{}, "replay_id"))
			assertutil.AssertEqualIgnoring(t, test.wantBrdcasts, brdCastOutputs, protocmp.Transform(), protocmp.IgnoreFields(&pb.ForfeitOutput{}, "replay_id"))

			if test.assertDatabases != nil {
				test.assertDatabases(t, &databases, msgOutputs)
			}
		})
	}
}
