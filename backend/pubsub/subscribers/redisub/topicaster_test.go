package redisub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastBroker(t *testing.T) {
	broker := NewTopicBroadcaster("test-broadcast", DefaultBrokerConfig)
	defer broker.Close()

	wantSub1Msgs := []string{"test1", "test2", "test3"}
	wantSub2Msgs := []string{"test2", "test3"}
	wantSub3Msgs := []string{"test2"}
	wantSub4Msgs := []string{"test4"}

	mChan1 := make(chan [][]byte)
	mChan2 := make(chan [][]byte)
	mChan3 := make(chan [][]byte)
	mChan4 := make(chan [][]byte)

	sub1, unsub1 := broker.Subscribe("1", len(wantSub1Msgs))
	sub4, unsub4 := broker.Subscribe("2", len(wantSub4Msgs))

	go testSubscriber(sub1, mChan1)
	go testSubscriber(sub4, mChan4)

	broker.Publish("1", []byte("test1"))

	sub2, unsub2 := broker.Subscribe("1", len(wantSub2Msgs))
	sub3, unsub3 := broker.Subscribe("1", len(wantSub3Msgs))

	go testSubscriber(sub2, mChan2)
	go testSubscriber(sub3, mChan3)

	broker.Publish("1", []byte("test2"))

	unsub3()

	broker.Publish("1", []byte("test3"))

	broker.Publish("2", []byte("test4"))

	unsub1()
	unsub2()
	unsub4()

	assert.Equal(t, wantSub1Msgs, getMessages(t, mChan1))
	assert.Equal(t, wantSub2Msgs, getMessages(t, mChan2))
	assert.Equal(t, wantSub3Msgs, getMessages(t, mChan3))
	assert.Equal(t, wantSub4Msgs, getMessages(t, mChan4))
}
