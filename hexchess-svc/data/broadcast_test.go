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

func TestBroadcastGameMessage(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	m := MakeMultiCasterMap("testing-broker-map", time.Hour*1)
	<-ListenGameMessages(m, rdb.PubsubAddr)

	expMsgCount := 2

	makeTestChatOutput := func(id string, msg string) []byte {
		v, err := proto.Marshal(&pb.GameOutput{
			GameId: id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: msg}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	subChan := make(chan []byte)
	m.Subscribe("1", subChan)

	ctx := context.WithValue(context.Background(), util.Trace, "testing-broadcast-game-message")
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput("1", "test1")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput("1", "test2")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput("2", "test3")))

	var msgs []string
	for range expMsgCount {
		var o pb.GameOutput
		if err := proto.Unmarshal(<-subChan, &o); err != nil {
			t.Fatalf("unmarshal game output: %v", err)
		}
		msgs = append(msgs, o.GetChat().GetMessage())
	}

	assert.Equal(t, []string{"test1", "test2"}, msgs)
}
