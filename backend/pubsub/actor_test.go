package pubsub

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func testSubscriber[Event any](sub chan Event, mChan chan []Event) {
	var messages []Event
	for msg := range sub {
		messages = append(messages, msg)
	}
	mChan <- messages
}

func collectMessages(messages [][]byte) []string {
	var strMessages []string
	for _, msg := range messages {
		strMessages = append(strMessages, string(msg))
	}
	return strMessages
}

func TestMulticasterActor(t *testing.T) {
	actor := MakeBroadcastActor("testing-multicasters")
	defer actor.Shutdown()

	wantSub1Msgs := []string{"test1", "test2", "test3"}
	wantSub2Msgs := []string{"test2", "test3"}
	wantSub3Msgs := []string{"test2"}
	wantSub4Msgs := []string{"test4"}

	sub1 := make(chan []byte, len(wantSub1Msgs))
	sub2 := make(chan []byte, len(wantSub2Msgs))
	sub3 := make(chan []byte, len(wantSub3Msgs))
	sub4 := make(chan []byte, len(wantSub4Msgs))

	mChan1 := make(chan [][]byte)
	mChan2 := make(chan [][]byte)
	mChan3 := make(chan [][]byte)
	mChan4 := make(chan [][]byte)

	go testSubscriber(sub1, mChan1)
	go testSubscriber(sub2, mChan2)
	go testSubscriber(sub3, mChan3)
	go testSubscriber(sub4, mChan4)

	actor.Subscribe("1", sub1)
	actor.Subscribe("2", sub4)

	actor.Broadcast("1", []byte("test1"))

	actor.Subscribe("1", sub2)
	actor.Subscribe("1", sub3)

	actor.Broadcast("1", []byte("test2"))

	actor.Unsubscribe("1", sub3)
	actor.Broadcast("1", []byte("test3"))

	actor.Broadcast("2", []byte("test4"))

	actor.Unsubscribe("1", sub1)
	actor.Unsubscribe("1", sub2)
	actor.Unsubscribe("2", sub4)

	assert.Equal(t, wantSub1Msgs, collectMessages(<-mChan1))
	assert.Equal(t, wantSub2Msgs, collectMessages(<-mChan2))
	assert.Equal(t, wantSub3Msgs, collectMessages(<-mChan3))
	assert.Equal(t, wantSub4Msgs, collectMessages(<-mChan4))
}

func TestGlobalCasterActor(t *testing.T) {
	actor := MakeGlobalCasterActor("testing-globalcaster")
	defer actor.Shutdown()

	e1 := GlobalCastEvent{Kind: 0, Data: "test1"}
	e2 := GlobalCastEvent{Kind: 1, Data: "test2"}
	wantSub1Events := []GlobalCastEvent{e1, e2}
	wantSub2Events := []GlobalCastEvent{e2}

	sub1 := make(chan GlobalCastEvent, len(wantSub1Events))
	sub2 := make(chan GlobalCastEvent, len(wantSub2Events))

	mChan1 := make(chan []GlobalCastEvent)
	mChan2 := make(chan []GlobalCastEvent)

	go testSubscriber(sub1, mChan1)
	go testSubscriber(sub2, mChan2)

	actor.Subscribe(sub1)
	actor.Broadcast(e1)

	actor.Subscribe(sub2)
	actor.Broadcast(e2)

	actor.Unsubscribe(sub1)
	actor.Unsubscribe(sub2)

	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}
