package redisub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalBroadcastBroker(t *testing.T) {
	broker := NewGlobalBroadcaster("testing-broadcast-global", DefaultBrokerConfig)
	defer broker.Close()

	e1 := []byte("test1")
	e2 := []byte("test2")
	wantSub1Events := [][]byte{e1, e2}
	wantSub2Events := [][]byte{e2}

	mChan1 := make(chan [][]byte)
	mChan2 := make(chan [][]byte)

	sub1, unsub1 := broker.Subscribe(len(wantSub1Events))

	go testSubscriber(sub1, mChan1)

	broker.Broadcast(e1)

	sub2, unsub2 := broker.Subscribe(len(wantSub2Events))

	go testSubscriber(sub2, mChan2)

	broker.Broadcast(e2)

	unsub1()
	unsub2()

	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}
