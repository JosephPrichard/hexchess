package subscribers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalBroadcastBroker(t *testing.T) {
	c := make(chan struct{}, 1)
	config := DefaultBrokerConfig
	config.outputChan = c
	config.GlobalBrokerShardCount = 6

	broker := NewGlobalBroadcastBroker[GlobalCastEvent]("testing-broadcast-global", config)
	defer broker.Shutdown()

	e1 := GlobalCastEvent{Kind: 0, Data: "test1"}
	e2 := GlobalCastEvent{Kind: 1, Data: "test2"}
	wantSub1Events := []GlobalCastEvent{e1, e2}
	wantSub2Events := []GlobalCastEvent{e2}

	mChan1 := make(chan []GlobalCastEvent)
	mChan2 := make(chan []GlobalCastEvent)

	sub1, unsub1 := broker.Subscribe(len(wantSub1Events))
	get(t, c, 1)

	go testSubscriber(sub1.Mailbox(), mChan1)

	broker.Broadcast(e1)
	get(t, c, config.GlobalBrokerShardCount)

	sub2, unsub2 := broker.Subscribe(len(wantSub2Events))
	get(t, c, 1)

	go testSubscriber(sub2.Mailbox(), mChan2)

	broker.Broadcast(e2)
	get(t, c, config.GlobalBrokerShardCount)

	unsub1()
	get(t, c, 1)
	unsub2()
	get(t, c, 1)

	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}
