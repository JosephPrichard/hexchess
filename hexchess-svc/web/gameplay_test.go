package web

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/out"
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
		{
			GameId: gameID,
			Value:  &pb.GameOutput_BgInit{BgInit: &pb.BgInitOutput{}},
		},
	}
	cmnBrdcasts := []any{cmnWantMsgs[1], cmnWantMsgs[2]}

	//ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

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
				{
					GameId: gameID,
					Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
						UpdatedAt: db.TestTimeNow.Format(time.RFC3339),
						Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, CollFile: true, CollRank: true},
					}},
				},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{
					GameId: gameID,
					Value: &pb.GameOutput_Move{Move: &pb.MoveOutput{
						UpdatedAt: db.TestTimeNow.Format(time.RFC3339),
						Move:      &pb.HistMove{Piece: int32(chess.WhitePawn), FromRank: 0, FromFile: 1, ToFile: 1, ToRank: 1, CollFile: true, CollRank: true},
					}},
				}},
		},
		{
			name: "chat input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}},
			},
			wantMsgs: []*pb.GameOutput{
				{
					GameId: gameID,
					Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
						Message: "Hello World",
						Player:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
						SentAt:  db.TestTimeNow.Format(time.RFC3339),
					}},
				},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{
					GameId: gameID,
					Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
						Message: "Hello World",
						Player:  &pb.PlayerState{Id: 1, Name: "user1", Country: "us"},
						SentAt:  db.TestTimeNow.Format(time.RFC3339),
					}},
				},
			},
		},
		{
			name: "forfeit input",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Forfeit{}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Forfeit{}},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Forfeit{}},
			},
		},
		{
			name: "undo input (success)",
			inputMsgs: []*pb.GameInput{
				{Value: &pb.GameInput_Undo{Undo: &pb.UndoInput{Kind: "CREATE"}}},
			},
			wantMsgs: []*pb.GameOutput{
				{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "CREATE", UndoId: 1}}},
			},
			wantBrdcasts: []any{
				&pb.GameOutput{GameId: gameID, Value: &pb.GameOutput_Undo{Undo: &pb.UndoOutput{Kind: "CREATE", UndoId: 1}}},
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
			wantMsgs := slices.Concat(cmnWantMsgs, test.wantMsgs)
			wantBrdcasts := slices.Concat(cmnBrdcasts, test.wantBrdcasts)

			state, closer := svc.BeforeStateTest(t, true)
			defer closer()

			state.EntropySource = &out.StableSource{Time: db.TestTimeNow}
			setup := Setup{State: state, Broadcasters: svc.MakeBroadcaster()}

			createTestSessions(t, state)
			createTestChessStates(t, state)

			ts := httptest.NewServer(MakeRoot(setup))
			defer ts.Close()

			// (start, subcribe, and read broadcasts)
			<-setup.Broadcasters.ListenGameMessages(state.Redis)
			subChan := make(chan []byte)
			setup.Broadcasters.GamesCaster.Subscribe(gameID, subChan)
			brdCastChan := make(chan []any)
			go readBroadcasted(brdCastChan, subChan, len(wantBrdcasts))

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
			msgOutputs := make([]*pb.GameOutput, len(wantMsgs))
			for i := range wantMsgs {
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
			assertutil.ElementsMatch(t, wantMsgs, msgOutputs, cmpOpts...)
			assertutil.ElementsMatch(t, wantBrdcasts, brdCastOutputs, cmpOpts...)
		})
	}
}
