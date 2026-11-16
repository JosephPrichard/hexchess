package pubsub

import (
	"hexchess-svc/cache"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type SubscriberAPI interface {
	Mailbox() chan []byte
}

type ExpectBroadcastResult struct {
	Assert   func()
	Shutdown func()
}

func ExpectBroadcast[Message any](
	t *testing.T,
	rdb cache.Redis,
	want []Message,
	subscribe func(lb *subscribers.LocalBroadcasters, capacity int) SubscriberAPI,
	unmarshal func(bytes []byte) Message,
	cmpOpts ...cmp.Option,
) ExpectBroadcastResult {
	t.Helper()

	if want == nil {
		return ExpectBroadcastResult{Assert: func() {}, Shutdown: func() {}}
	}

	lb := subscribers.NewLocalBroadcasters(subscribers.MinimalBrokerConfig)
	lb.Listen(t.Context(), rdb) // start listening before this function returns

	subscriber := subscribe(lb, len(want))

	// closure should be executed after the operation that will trigger notifications
	assert := func() {
		t.Helper()

		for i := range want {
			var raw []byte
			select {
			case raw = <-subscriber.Mailbox():
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for broadcaster message %d/%d", i+1, len(want))
			}

			got := unmarshal(raw)

			testutil.Equal(t, want[i], got, cmpOpts...)
		}
	}

	shutdown := func() {
		lb.Shutdown() // close the broadcasters only after running assertions (ideally at the end with a `defer` call)
	}
	return ExpectBroadcastResult{Assert: assert, Shutdown: shutdown}
}
