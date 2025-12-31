package web

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
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

// TestHandleGameplayWs is a high-level black box test that checks the broadcast and websocket output for every input case
// Database assertions run after message assertions and can assume that inbound websocket messages are valid
func TestHandleGameplayWs(t *testing.T) {
	gameID := TestGameID1
	cmnWantMsgs := []*pb.GameOutput{
		{
			GameId: gameID,
			Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
				State: &pb.ChessState{
					Id: gameID,
					// ignoring Game and Touch.
					WhitePlayer:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
					BlackPlayer:  &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
					IsEnded:      false,
					FirstColor:   svc.Random.String(),
					Mode:         svc.ModeCorrespondence1.String(),
					InitialBoard: nil,
					UndoId:       0,
				},
				Self: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			}},
		},
		{
			GameId: gameID,
			Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
				BlackPlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
				WhitePlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			}},
		},
	}

	ctx := context.WithValue(t.Context(), logutil.Trace, "test-handle-gameplay-ws")

	for _, test := range []struct {
		name            string
		inputMsgs       []*pb.GameInput
		wantMsgs        []*pb.GameOutput
		wantBrdcasts    []any
		assertDatabases func(*testing.T, *db.Databases, []*pb.GameOutput)
	}{
		{
			name: "move input (invalid)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{}}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{
					GameId: gameID,
					Value:  &pb.GameOutput_Error{Error: &pb.ErrorOutput{Message: ErrWsInvalidMove.Error()}},
				},
			}),
		},
		{
			name: "move input (valid)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: chess.PbMoveStr("b1", "b2")}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{
					GameId: gameID,
					Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
						UpdatedAt: svc.TestTimeNow.Format(time.RFC3339),
						Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, CollFile: true, CollRank: true},
					}},
				},
			}),
			wantBrdcasts: []any{
				&pb.GameOutput{
					GameId: gameID,
					Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
						UpdatedAt: svc.TestTimeNow.Format(time.RFC3339),
						Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, CollFile: true, CollRank: true},
					}},
				}},
		},
		{
			name: "chat input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: "Hello World"}}},
			}),
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: "Hello World"}}},
			},
		},
		{
			name: "forfeit input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Forfeit{}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Forfeit{}},
			}),
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Forfeit{}},
			},
			assertDatabases: func(t *testing.T, databases *db.Databases, msgOutputs []*pb.GameOutput) {
				replayID := msgOutputs[2].GetForfeit().ReplayId // we can assume this is valid if the test reaches this point

				actualState, err := svc.GetChessState(ctx, databases.Rdb, gameID)
				require.NoError(t, err)
				assert.True(t, actualState.IsEnded)

				replay, err := databases.Query.SelectReplayRowByID(context.Background(), replayID)
				require.NoError(t, err)
				assert.Equal(t, replay.Cause, db.CauseEnum("FORFEIT"))
			},
		},
		{
			name: "undo input (success)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "CREATE"}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "CREATE", UndoId: 1}}},
			}),
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "CREATE", UndoId: 1}}},
			},
		},
		{
			name: "undo input (fail)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "ACCEPT"}}},
			},
			wantMsgs: slices.Concat(cmnWantMsgs, []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{Message: ErrWsUndoAction.Error()}}},
			}),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			databases, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			createTestSessions(t, databases.Rdb)
			createTestChessStates(t, databases.Rdb)

			state := State{Databases: databases, Broadcasters: svc.MakeBroadcaster(), Generators: &outbound.StableGenerator{Time: svc.TestTimeNow}}
			ts := httptest.NewServer(HandleRoot(state))
			defer ts.Close()

			// (start, subcribe, and read broadcasts)
			<-state.Broadcasters.ListenGameMessages(databases.Rdb)
			subChan := make(chan []byte)
			state.Broadcasters.GamesCaster.Subscribe(gameID, subChan)
			brdCastChan := make(chan []any)
			go readBroadcasted(brdCastChan, subChan, len(test.wantBrdcasts))

			// when
			url := strings.Replace(fmt.Sprintf("%s/api/ws/game?gameId=%s&sessionId=%s", ts.URL, gameID, TestSessionID1), "http", "ws", 1)
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

			cmpOpts := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.ChessState{}, "touch", "game", "initial_board"),
				protocmp.IgnoreFields(&pb.MoveOutput{}, "game"),
				protocmp.IgnoreFields(&pb.UndoOutput{}, "game"),
				protocmp.IgnoreFields(&pb.ForfeitOutput{}, "replay_id"),
			}
			assertutil.AssertEqualIgnoring(t, test.wantMsgs, msgOutputs, cmpOpts...)
			assertutil.AssertEqualIgnoring(t, test.wantBrdcasts, brdCastOutputs, cmpOpts...)

			if test.assertDatabases != nil {
				test.assertDatabases(t, &databases, msgOutputs)
			}
		})
	}
}
