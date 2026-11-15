package subscribers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalBroadcastBroker(t *testing.T) {
	broker := NewGlobalBroadcastBroker[GlobalCastEvent]("testing-broadcast-global", DefaultBrokerConfig)
	defer broker.Shutdown()

	e1 := GlobalCastEvent{Kind: 0, Data: "test1"}
	e2 := GlobalCastEvent{Kind: 1, Data: "test2"}
	wantSub1Events := []GlobalCastEvent{e1, e2}
	wantSub2Events := []GlobalCastEvent{e2}

	mChan1 := make(chan []GlobalCastEvent)
	mChan2 := make(chan []GlobalCastEvent)

	sub1 := broker.Subscribe(len(wantSub1Events))

	go testSubscriber(sub1.Mailbox(), mChan1)

	broker.Broadcast(e1)

	sub2 := broker.Subscribe(len(wantSub2Events))

	go testSubscriber(sub2.Mailbox(), mChan2)

	broker.Broadcast(e2)

	broker.Unsubscribe(sub1)
	broker.Unsubscribe(sub2)

	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}
