package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/itest"
	"hexchess-svc/utils/async"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type testEventHandler struct {
	lock           sync.Mutex
	cancel         func()
	outputEvents   []testEvent
	wantEventCount int
}

func (h *testEventHandler) handleEvent(_ context.Context, bytes []byte) error {
	var e testEvent
	if err := json.Unmarshal(bytes, &e); err != nil {
		return err
	}

	h.lock.Lock()
	h.outputEvents = append(h.outputEvents, e)
	h.lock.Unlock()

	if len(h.outputEvents) == h.wantEventCount {
		h.cancel()
	}
	return nil
}

func TestRedisConsumer(t *testing.T) {
	consumingStream := "stream-key"

	marshal := func(v any) []byte {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		return data
	}

	tests := []struct {
		name          string
		inputStream   string
		inputEvents   []map[string]any
		partitionKeys []string
		wantEvents    []testEvent
	}{
		{
			name:          "ConsumesEvents",
			inputStream:   fmt.Sprintf("%s:{0}", consumingStream),
			partitionKeys: []string{"0", "1"},
			inputEvents: []map[string]any{
				{
					"data": "invalid",
				},
				{
					"unknown": "field",
				},
				{
					"data": marshal(testEvent{
						Key:   "KeyOne",
						Value: "ValueOne",
					}),
				},
				{
					"data": marshal(testEvent{
						Key:   "KeyTwo",
						Value: "ValueTwo",
					}),
				},
			},
			wantEvents: []testEvent{
				{
					Key:   "KeyOne",
					Value: "ValueOne",
				},
				{
					Key:   "KeyTwo",
					Value: "ValueTwo",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testinfra := itest.SetupIntegrationTest(t, itest.Redis, itest.RWPostgres)
			defer testinfra.Close()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			inputEvents := append([]map[string]any{}, tt.inputEvents...)

			for _, event := range inputEvents {
				xArgs := &redis.XAddArgs{
					Stream: tt.inputStream,
					Values: event,
				}
				err := testinfra.Redis.PrimaryClient.XAdd(ctx, xArgs).Err()
				require.NoError(t, err)
			}

			h := testEventHandler{cancel: cancel, wantEventCount: len(tt.wantEvents)}
			consumer := StreamConsumer{
				ctx:        ctx,
				redis:      testinfra.Redis.PrimaryClient,
				querier:    testinfra.QuerierMutator(),
				dispatcher: async.SyncDispatcher{},

				consumeFunc: h.handleEvent,

				streamKey:     consumingStream,
				consumerGroup: "consumer-group",
				pollCount:     8,
				partitionKeys: tt.partitionKeys,
				blockDuration: time.Millisecond,
			}
			consumer.Consume()

			assert.ElementsMatch(t, tt.wantEvents, h.outputEvents)
		})
	}
}
