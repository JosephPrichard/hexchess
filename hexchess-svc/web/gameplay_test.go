package web

import (
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/ptr"
	"hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func writeMessage(t *testing.T, conn *websocket.Conn, pbInput *pb.GameInput) {
	b, err := proto.Marshal(pbInput)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

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

func TestHandleGameplayWs(t *testing.T) {
	state := svc.MakeState(svc.StateSetup{
		ID:         TestGameID1,
		Mode:       svc.ModeCorrespondence1,
		FirstColor: svc.Random,
		White:      ptr.New(svc.MakePlayer(2, "user2", "us")),
		Black:      ptr.New(svc.MakePlayer(1, "user1", "us")),
	})
	commonMsgs := []*pb.GameOutput{
		{
			GameId: TestGameID1,
			Value: &pb.GameOutput_Init{Init: &pb.InitOutput{
				Self:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
				State: svc.SerializeChessState(ptr.New(state)),
			}},
		},
		{
			GameId: TestGameID1,
			Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
				WhitePlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us"},
				BlackPlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
			}},
		},
	}

	for _, test := range []struct {
		name         string
		inputMsgs    []*pb.GameInput
		wantMsgs     []*pb.GameOutput
		wantBrdcasts []any
	}{
		{
			name: "move input (invalid turn)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{FromFile: 1, FromRank: 1, ToFile: 2, ToRank: 1}}}},
			},
			wantMsgs: slices.Concat(commonMsgs, []*pb.GameOutput{
				{
					GameId: TestGameID1,
					Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
						Message: ErrWsTurn.Error(),
					}},
				},
			}),
		},
		{
			name: "chat input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}},
			},
			wantMsgs: slices.Concat(commonMsgs, []*pb.GameOutput{
				{
					GameId: TestGameID1,
					Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
						Message: "Hello World",
					}},
				},
			}),
			wantBrdcasts: []any{
				&pb.GameOutput{
					GameId: TestGameID1,
					Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
						Message: "Hello World",
					}},
				},
			},
		},
		{
			name: "forfeit",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Forfeit{}},
			},
			//wantMsgs: slices.Concat(commonMsgs, []*pb.GameOutput{
			//	{
			//		GameId: TestGameID1,
			//		Value:  &pb.GameOutput_Forfeit{},
			//	},
			//}),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			dbs, closer := db.BeforeDbTest(t, true, svc.InsertTestData)
			defer closer()

			createTestSessions(t, dbs.Rdb)
			createTestChessStates(t, dbs.Rdb)

			state := State{Databases: dbs, Broadcasters: svc.MakeBroadcaster()}

			ts := httptest.NewServer(HandleRoot(state))
			defer ts.Close()

			// (start, subcribe, and read broadcasts)
			<-state.Broadcasters.ListenGameMessages(dbs.Rdb)
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
				writeMessage(t, conn, input)
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

			assertutil.AssertEqualIgnoring(t, test.wantMsgs, msgOutputs, protocmp.Transform(), protocmp.IgnoreFields(&pb.ChessState{}, "touch"))
			assertutil.AssertEqualIgnoring(t, test.wantBrdcasts, brdCastOutputs, protocmp.Transform())
		})
	}
}
