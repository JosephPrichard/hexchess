package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestMultiCaster(t *testing.T) {
	testSub := func(sub chan []byte, mChan chan []string) {
		var messages []string
		for msg := range sub {
			messages = append(messages, string(msg))
		}
		t.Logf("completed testing subscriber: %v: %v", sub, messages)
		mChan <- messages
	}

	m := MakeMultiCasterMap("testing-mc", time.Hour*1)

	sub1 := make(chan []byte)
	sub2 := make(chan []byte)
	sub3 := make(chan []byte)
	sub4 := make(chan []byte)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)
	mChan3 := make(chan []string)
	mChan4 := make(chan []string)

	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)
	go testSub(sub3, mChan3)
	go testSub(sub4, mChan4)

	m.Subscribe("1", sub1)
	m.Subscribe("2", sub4)
	m.Broadcast("1", []byte("test1"))

	m.Subscribe("1", sub2)
	m.Subscribe("1", sub3)
	m.Broadcast("1", []byte("test2"))

	m.Unsubscribe("1", sub3)
	m.Broadcast("1", []byte("test3"))

	m.Broadcast("2", []byte("test4"))

	m.Unsubscribe("1", sub1)
	m.Unsubscribe("1", sub2)
	m.Unsubscribe("2", sub4)

	assert.Equal(t, []string{"test1", "test2", "test3"}, <-mChan1)
	assert.Equal(t, []string{"test2", "test3"}, <-mChan2)
	assert.Equal(t, []string{"test2"}, <-mChan3)
	assert.Equal(t, []string{"test4"}, <-mChan4)
}

func TestUnicaster(t *testing.T) {
	testSub := func(sub chan UcEvent, mChan chan []UcEvent) {
		var messages []UcEvent
		for msg := range sub {
			messages = append(messages, msg)
		}
		t.Logf("completed testing subscriber: %v: %v", sub, messages)
		mChan <- messages
	}

	m := MakeUniCaster("testing-uc")

	sub1 := make(chan UcEvent)
	sub2 := make(chan UcEvent)

	mChan1 := make(chan []UcEvent)
	mChan2 := make(chan []UcEvent)

	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)

	e1 := UcEvent{Kind: 0, Data: "test1"}
	e2 := UcEvent{Kind: 1, Data: "test2"}

	m.Subscribe(sub1)
	m.Broadcast(e1)

	m.Subscribe(sub2)
	m.Broadcast(e2)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	assert.Equal(t, []UcEvent{e1, e2}, <-mChan1)
	assert.Equal(t, []UcEvent{e2}, <-mChan2)
}

func getChatMessages(t *testing.T, bytes [][]byte) []string {
	var msgs []string
	for _, b := range bytes {
		var output pb.GameOutput
		assert.NoError(t, proto.Unmarshal(b, &output))
		msgs = append(msgs, output.GetChat().GetMessage())
	}
	return msgs
}

func testCountSub(t *testing.T, sub chan []byte, mChan chan [][]byte, count int) {
	var messages [][]byte
	for range count {
		msg := <-sub
		messages = append(messages, msg)
	}
	t.Logf("completed testing subscriber: %v: %v", sub, messages)
	mChan <- messages
}

func makeTestChatOutput(t *testing.T, id string, msg string) []byte {
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: id,
		Value:  &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: msg}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBroadcastGameMessage(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close() // this will also stop the goroutine listening to the pubsub channel

	m := MakeMultiCasterMap("testing-broker-map", time.Hour*1)
	<-ListenGameMessages(m, rdb.PubsubAddr)

	ctx := context.WithValue(context.Background(), util.Trace, "testing-broadcast-game-message")

	sub := make(chan []byte)
	mChan := make(chan [][]byte)
	go testCountSub(t, sub, mChan, 2)
	m.Subscribe("1", sub)

	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "1", "test1")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "1", "test2")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "2", "test3")))

	assert.Equal(t, []string{"test1", "test2"}, getChatMessages(t, <-mChan))
}
