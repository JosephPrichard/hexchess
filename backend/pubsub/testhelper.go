package pubsub

import (
	"hexchess-svc/itest"
	"hexchess-svc/pubsub/subscribers/redisub"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type ExpectBroadcastResult struct {
	Assert func()
	Close  func()
}

func ExpectBroadcast[Message any](
	t *testing.T,
	infra itest.TestInfra,
	want []Message,
	subscribe func(s Subscriber) <-chan []byte,
	unmarshal func(bytes []byte) Message,
	cmpOpts ...cmp.Option,
) ExpectBroadcastResult {
	t.Helper()

	if want == nil {
		return ExpectBroadcastResult{Assert: func() {}, Close: func() {}}
	}

	subscriber := redisub.NewRedisBroadcastBroker(redisub.MinimalBrokerConfig, infra.Redis.PubSubClient)
	subscriber.Start(t.Context()) // start listening before this function returns

	//subscriber := &natsub.NatsSubscriber{}

	sChan := subscribe(subscriber)

	// closure should be executed after the operation that will trigger notifications
	assert := func() {
		t.Helper()

		for i := range want {
			var raw []byte
			select {
			case raw = <-sChan:
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for broadcaster message %d/%d", i+1, len(want))
			}

			got := unmarshal(raw)

			testutil.Equal(t, want[i], got, cmpOpts...)
		}
	}
	shutdown := func() {
		subscriber.Close() // close the broadcasters only after running assertions (ideally at the end with a `defer` call)
	}

	return ExpectBroadcastResult{Assert: assert, Close: shutdown}
}
