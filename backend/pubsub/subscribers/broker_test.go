package subscribers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastBroker(t *testing.T) {
	c := make(chan struct{}, 1)
	config := DefaultBrokerConfig
	config.outputChan = c

	broker := NewBroadcastBroker[[]byte]("test-broadcast", config)
	defer broker.Shutdown()

	wantSub1Msgs := []string{"test1", "test2", "test3"}
	wantSub2Msgs := []string{"test2", "test3"}
	wantSub3Msgs := []string{"test2"}
	wantSub4Msgs := []string{"test4"}

	mChan1 := make(chan [][]byte)
	mChan2 := make(chan [][]byte)
	mChan3 := make(chan [][]byte)
	mChan4 := make(chan [][]byte)

	sub1, unsub1 := broker.Subscribe("1", len(wantSub1Msgs))
	get(t, c, 1)
	sub4, unsub4 := broker.Subscribe("2", len(wantSub4Msgs))
	get(t, c, 1)

	go testSubscriber(sub1.Mailbox(), mChan1)
	go testSubscriber(sub4.Mailbox(), mChan4)

	broker.Broadcast("1", []byte("test1"))
	get(t, c, 1)

	sub2, unsub2 := broker.Subscribe("1", len(wantSub2Msgs))
	get(t, c, 1)
	sub3, unsub3 := broker.Subscribe("1", len(wantSub3Msgs))
	get(t, c, 1)

	go testSubscriber(sub2.Mailbox(), mChan2)
	go testSubscriber(sub3.Mailbox(), mChan3)

	broker.Broadcast("1", []byte("test2"))
	get(t, c, 1)

	unsub3()
	get(t, c, 1)

	broker.Broadcast("1", []byte("test3"))
	get(t, c, 1)

	broker.Broadcast("2", []byte("test4"))
	get(t, c, 1)

	unsub1()
	get(t, c, 1)
	unsub2()
	get(t, c, 1)
	unsub4()
	get(t, c, 1)

	assert.Equal(t, wantSub1Msgs, getMessages(t, mChan1))
	assert.Equal(t, wantSub2Msgs, getMessages(t, mChan2))
	assert.Equal(t, wantSub3Msgs, getMessages(t, mChan3))
	assert.Equal(t, wantSub4Msgs, getMessages(t, mChan4))
}
