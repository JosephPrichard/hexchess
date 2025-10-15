package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/app/util"
	"hexchess-svc/pb"
	"testing"
)

func testStrSub(t *testing.T, sub subscriber, mChan chan []string) {
	var messages []string
	for msg := range sub {
		messages = append(messages, string(msg))
	}
	t.Logf("completed test subscriber: %v: %v", sub, messages)
	mChan <- messages
}

func TestMultiBroker(t *testing.T) {
	m := MakeMultiBrokerMap("test-broker")

	sub1 := make(subscriber)
	sub2 := make(subscriber)
	sub3 := make(subscriber)
	sub4 := make(subscriber)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)
	mChan3 := make(chan []string)
	mChan4 := make(chan []string)

	go testStrSub(t, sub1, mChan1)
	go testStrSub(t, sub2, mChan2)
	go testStrSub(t, sub3, mChan3)
	go testStrSub(t, sub4, mChan4)

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

func TestSingleBroker(t *testing.T) {
	m := MakeSingleBroker("test-broker")

	sub1 := make(subscriber)
	sub2 := make(subscriber)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)

	go testStrSub(t, sub1, mChan1)
	go testStrSub(t, sub2, mChan2)

	m.Subscribe(sub1)
	m.Broadcast([]byte("test1"), BrokerExpireTime)

	m.Subscribe(sub2)
	m.Broadcast([]byte("test2"), BrokerExpireTime)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	assert.Equal(t, []string{"test1", "test2"}, <-mChan1)
	assert.Equal(t, []string{"test2"}, <-mChan2)
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

func testCountSub(t *testing.T, sub subscriber, mChan chan [][]byte, count int) {
	var messages [][]byte
	for range count {
		msg := <-sub
		messages = append(messages, msg)
	}
	t.Logf("completed test subscriber: %v: %v", sub, messages)
	mChan <- messages
}

func TestBroadcastGameMessage(t *testing.T) {
	rdb, addr, closer := beforeRedisTestsWithAddr(t)
	defer closer() // this will also stop the goroutine listening to the pubsub channel

	m := MakeMultiBrokerMap("test-broker-map")
	DialAndListenGameMessages(m, addr)

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-broadcast-game-message")

	sub := make(subscriber)
	mChan := make(chan [][]byte)
	go testCountSub(t, sub, mChan, 2)
	m.Subscribe("1", sub)

	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChat("1", "test1")))
	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChat("1", "test2")))
	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChat("2", "test3")))

	assert.Equal(t, []string{"test1", "test2"}, getChatMessages(t, <-mChan))
}

func BenchmarkBroadcastGameMessage(b *testing.B) {
	rdb, addr, closer := beforeRedisTestsWithAddr(b)
	defer closer() // this will also stop the goroutine listening to the pubsub channel

	m := MakeMultiBrokerMap("test-broker-map")
	DialAndListenGameMessages(m, addr)

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-broadcast-game-message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, BroadcastGameMessage(ctx, rdb, pb.MakeChat("1", "test1")))
	}
}
