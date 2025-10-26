package web

import (
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"hexchess-svc/pb"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func writeMessage(t *testing.T, conn *websocket.Conn, pbInput *pb.GameInput) {
	b, err := proto.Marshal(pbInput)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}
}

func readOutputs(outputsChan chan []any, subChan chan []byte, count int) {
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
	gameID := "game1"
	expCount1 := 4
	expCount2 := 1

	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	createTestSessions(t, stores.Rdb)
	createTestChessStates(t, stores.Rdb)

	state := MakeServerState(stores, nil, nil)
	data.ListenGameMessages(state.GamesCaster, stores.Rdb.Addr)

	ts := httptest.NewServer(HandleRoot(state))
	defer ts.Close()

	subChan := make(chan []byte)
	state.GamesCaster.Subscribe(gameID, subChan)
	outputs2Chan := make(chan []any)
	go readOutputs(outputs2Chan, subChan, expCount2)

	url := strings.Replace(ts.URL+"/api/ws/game?id="+gameID, "http", "ws", 1)
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
		"Cookie": []string{FmtCookie(sessionID1)},
	})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	writeMessage(t, conn, &pb.GameInput{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.PieceMove{FromFile: 1, FromRank: 1, ToFile: 2, ToRank: 1}}}})
	writeMessage(t, conn, &pb.GameInput{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}})

	outputs1 := make([]pb.GameOutput, expCount1)
	for i := range expCount1 {
		_, b, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read ws message: %v", err)
		}
		if err := proto.Unmarshal(b, &outputs1[i]); err != nil {
			t.Fatalf("failed to marshal input: %v", err)
		}
	}
	outputs2 := <-outputs2Chan

	expected1 := []pb.GameOutput{
		{
			GameId: gameID,
			Value:  &pb.GameOutput_Init{Init: &pb.InitOutput{}}, // this information is too complex to assert in this test.
		},
		{
			GameId: gameID,
			Value: &pb.GameOutput_Players{Players: &pb.PlayersOutput{
				WhitePlayer: &pb.PlayerState{Id: 2, Name: "user2", Country: "us", Elo: 1000, IsGuest: false},
				BlackPlayer: &pb.PlayerState{Id: 1, Name: "user1", Country: "us", Elo: 1000, IsGuest: false},
			}},
		},
		{
			GameId: gameID,
			Value: &pb.GameOutput_Error{Error: &pb.ErrorOutput{
				Message: ErrWsTurn.Error(),
			}},
		},
		{
			GameId: gameID,
			Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
				Message: "Hello World",
			}},
		},
	}
	expected2 := []any{
		&pb.GameOutput{
			GameId: gameID,
			Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
				Message: "Hello World",
			}},
		},
	}
	lib.AssertEqualIgnoring(t, expected1, outputs1, protocmp.Transform(), protocmp.IgnoreFields(&pb.InitOutput{}, "state", "self"))
	lib.AssertEqualIgnoring(t, expected2, outputs2, protocmp.Transform())
}
