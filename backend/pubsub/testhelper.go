package pubsub

import (
	"context"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub/subscribers/redisub"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

type ExpectBroadcastResult struct {
	Assert func()
	Close  func()
}

func GetTestSubscriber(ctx context.Context, infra itest.TestInfra) Subscriber {
	//subscriber := &natsub.NatsSubscriber{}
	//return subscriber

	subscriber := redisub.NewRedisBroadcastBroker(redisub.DefaultBrokerConfig, infra.Redis.PubSubClient)
	subscriber.Start(ctx)
	return subscriber
}

func ExpectBroadcast[Message any](
	t *testing.T,
	infra itest.TestInfra,
	channel string,
	topic string,
	want []Message,
	unmarshal func(bytes []byte) Message,
	cmpOpts ...cmp.Option,
) ExpectBroadcastResult {
	t.Helper()
	ctx := t.Context()

	subscriber := GetTestSubscriber(ctx, infra)

	sChan, unsubscribe, err := subscriber.Subscribe(ctx, channel, topic)
	require.NoError(t, err)

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
		unsubscribe()
		subscriber.Close()
	}

	return ExpectBroadcastResult{Assert: assert, Close: shutdown}
}
