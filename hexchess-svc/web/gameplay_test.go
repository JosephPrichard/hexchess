package web

import (
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
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
	// given
	gameID := "game1"
	expCount1 := 4
	expCount2 := 1

	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	createTestSessions(t, rdb)
	createTestChessStates(t, rdb)

	state := MakeServerState(ServerSetup{Databases: db.Databases{Rdb: rdb}})
	<-svc.ListenGameMessages(state.GamesCaster, rdb)

	ts := httptest.NewServer(HandleRoot(state, ""))
	defer ts.Close()

	subChan := make(chan []byte)
	state.GamesCaster.Subscribe(gameID, subChan)

	brdCastChan := make(chan []any)
	go readBroadcasted(brdCastChan, subChan, expCount2)

	// when
	url := strings.Replace(fmt.Sprintf("%s/api/ws/game?gameId=%s&sessionId=%s", ts.URL, gameID, TestSessionID1), "http", "ws", 1)
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	writeMessage(t, conn, &pb.GameInput{Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{FromFile: 1, FromRank: 1, ToFile: 2, ToRank: 1}}}})
	writeMessage(t, conn, &pb.GameInput{Value: &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello World"}}})

	// then
	msgOutputs := make([]pb.GameOutput, expCount1)
	for i := range expCount1 {
		_, b, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read ws message: %v", err)
		}
		if err := proto.Unmarshal(b, &msgOutputs[i]); err != nil {
			t.Fatalf("marshal input: %v", err)
		}
	}
	brdCastOutputs := <-brdCastChan

	expectedMsgs := []pb.GameOutput{
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
	expectedBrdCast := []any{
		&pb.GameOutput{
			GameId: gameID,
			Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
				Message: "Hello World",
			}},
		},
	}
	util.AssertEqualIgnoring(t, expectedMsgs, msgOutputs, protocmp.Transform(), protocmp.IgnoreFields(&pb.InitOutput{}, "state", "self"))
	util.AssertEqualIgnoring(t, expectedBrdCast, brdCastOutputs, protocmp.Transform())
}
