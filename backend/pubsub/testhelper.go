package pubsub

import (
	"hexchess-svc/cache"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func ExpectBroadcast[Message any](
	t *testing.T,
	rdb cache.Redis,
	want []Message,
	subscribe func(lb *subscribers.LocalBroadcasters, ch chan []byte),
	unmarshal func(bytes []byte) Message,
	cmpOpts ...cmp.Option,
) func() {
	t.Helper()

	if want == nil {
		return func() {}
	}

	lb := subscribers.NewLocalBroadcasters()
	lb.Listen(t.Context(), rdb)

	subChan := make(chan []byte, len(want))
	subscribe(lb, subChan)

	return func() {
		t.Helper()

		for i := range want {
			var raw []byte
			select {
			case raw = <-subChan:
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for broadcaster message %d/%d", i+1, len(want))
			}

			got := unmarshal(raw)

			testutil.Equal(t, want[i], got, cmpOpts...)
		}
	}
}
