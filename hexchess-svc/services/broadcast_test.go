package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

func TestMultiCaster(t *testing.T) {
	// given
	m := MakeMultiCasterMap("testing-mc", time.Hour*1)

	sub1 := make(chan []byte)
	sub2 := make(chan []byte)
	sub3 := make(chan []byte)
	sub4 := make(chan []byte)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)
	mChan3 := make(chan []string)
	mChan4 := make(chan []string)

	testSub := func(sub chan []byte, mChan chan []string) {
		var messages []string
		for msg := range sub {
			messages = append(messages, string(msg))
		}
		t.Logf("completed testing subscriber: %v: %v", sub, messages)
		mChan <- messages
	}
	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)
	go testSub(sub3, mChan3)
	go testSub(sub4, mChan4)

	// when
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

	// then
	assert.Equal(t, []string{"test1", "test2", "test3"}, <-mChan1)
	assert.Equal(t, []string{"test2", "test3"}, <-mChan2)
	assert.Equal(t, []string{"test2"}, <-mChan3)
	assert.Equal(t, []string{"test4"}, <-mChan4)
}

func TestUnicaster(t *testing.T) {
	// given
	m := MakeUniCaster("testing-uc")

	sub1 := make(chan UcEvent)
	sub2 := make(chan UcEvent)

	mChan1 := make(chan []UcEvent)
	mChan2 := make(chan []UcEvent)

	testSub := func(sub chan UcEvent, mChan chan []UcEvent) {
		var messages []UcEvent
		for msg := range sub {
			messages = append(messages, msg)
		}
		t.Logf("completed testing subscriber: %v: %v", sub, messages)
		mChan <- messages
	}
	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)

	e1 := UcEvent{Kind: 0, Data: "test1"}
	e2 := UcEvent{Kind: 1, Data: "test2"}

	// when
	m.Subscribe(sub1)
	m.Broadcast(e1)

	m.Subscribe(sub2)
	m.Broadcast(e2)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	// then
	assert.Equal(t, []UcEvent{e1, e2}, <-mChan1)
	assert.Equal(t, []UcEvent{e2}, <-mChan2)
}

func TestBroadcastGameMessage(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	m := MakeMultiCasterMap("testing-broker-map", time.Hour*1)
	<-ListenGameMessages(m, rdb)

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-broadcast-game-message")

	// when
	subChan := make(chan []byte)
	m.Subscribe("1", subChan)

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
	require.NoError(t, BroadcastMessage(ctx, rdb, rdb.GamesChan, makeTestChatOutput("1", "test1")))
	require.NoError(t, BroadcastMessage(ctx, rdb, rdb.GamesChan, makeTestChatOutput("1", "test2")))
	require.NoError(t, BroadcastMessage(ctx, rdb, rdb.GamesChan, makeTestChatOutput("2", "test3")))

	// then
	var messages []string
	for range 2 {
		var o pb.GameOutput
		if err := proto.Unmarshal(<-subChan, &o); err != nil {
			t.Fatalf("unmarshal game output: %v", err)
		}
		messages = append(messages, o.GetChat().GetMessage())
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}
