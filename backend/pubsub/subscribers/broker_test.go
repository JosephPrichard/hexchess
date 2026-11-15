package subscribers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastBroker(t *testing.T) {
	broker := NewBroadcastBroker[[]byte]("test-broadcast", DefaultBrokerConfig)
	defer broker.Shutdown()

	wantSub1Msgs := []string{"test1", "test2", "test3"}
	wantSub2Msgs := []string{"test2", "test3"}
	wantSub3Msgs := []string{"test2"}
	wantSub4Msgs := []string{"test4"}

	mChan1 := make(chan [][]byte)
	mChan2 := make(chan [][]byte)
	mChan3 := make(chan [][]byte)
	mChan4 := make(chan [][]byte)

	sub1 := broker.Subscribe("1", len(wantSub1Msgs))
	sub4 := broker.Subscribe("2", len(wantSub4Msgs))

	go testSubscriber(sub1.Mailbox(), mChan1)
	go testSubscriber(sub4.Mailbox(), mChan4)

	broker.Broadcast("1", []byte("test1"))

	sub2 := broker.Subscribe("1", len(wantSub2Msgs))
	sub3 := broker.Subscribe("1", len(wantSub3Msgs))

	go testSubscriber(sub2.Mailbox(), mChan2)
	go testSubscriber(sub3.Mailbox(), mChan3)

	broker.Broadcast("1", []byte("test2"))

	broker.Unsubscribe("1", sub3)
	broker.Broadcast("1", []byte("test3"))

	broker.Broadcast("2", []byte("test4"))

	broker.Unsubscribe("1", sub1)
	broker.Unsubscribe("1", sub2)
	broker.Unsubscribe("2", sub4)

	assert.Equal(t, wantSub1Msgs, collectMessages(<-mChan1))
	assert.Equal(t, wantSub2Msgs, collectMessages(<-mChan2))
	assert.Equal(t, wantSub3Msgs, collectMessages(<-mChan3))
	assert.Equal(t, wantSub4Msgs, collectMessages(<-mChan4))
}
