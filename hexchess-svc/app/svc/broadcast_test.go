package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
	"sync"
	"testing"
	"time"
)

func TestBroadcast_GameMessages(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-broadcast")

	m := StartListenGameMessages(rdb)

	sub1 := make(chan []byte)
	sub2 := make(chan []byte)
	sub3 := make(chan []byte)

	var wg sync.WaitGroup

	testSubscriber := func(sub chan []byte, expected ...string) {
		wg.Add(1)
		defer wg.Done()

		var messages []string

		timer := time.NewTimer(time.Second * 3)
		for range expected {
			select {
			case <-timer.C:
				t.Fatal("timed out in test subscriber")
			case msg := <-sub:
				var output pb.GameOutput
				assert.NoError(t, proto.Unmarshal(msg, &output))
				messages = append(messages, output.GetChat().GetMessage())
			}
		}

		t.Logf("messages: %v", messages)
		assert.Equal(t, expected, messages)
	}

	go testSubscriber(sub1, "test1", "test2", "test3")
	go testSubscriber(sub2, "test2", "test3")
	go testSubscriber(sub3, "test2")

	SubscribeBrokers(m, "1", sub1)
	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChatMessage("1", "test1")))

	SubscribeBrokers(m, "1", sub2)
	SubscribeBrokers(m, "1", sub3)
	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChatMessage("1", "test2")))

	UnsubscribeBrokers(m, "1", sub3)
	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChatMessage("1", "test3")))

	assert.NoError(t, BroadcastGameMessage(ctx, rdb, pb.MakeChatMessage("2", "test4")))

	wg.Wait()
}
