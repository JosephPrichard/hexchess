package consumers

import (
	"context"
	"encoding/json"
	"hexchess-svc/itest"
	"sync"
	"testing"

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
		go h.cancel()
	}
	return nil
}

func TestRedisConsumer(t *testing.T) {
	t.Parallel()

	inputEvents := []map[string]any{
		{
			"data": "invalid",
		},
		{
			"unknown": "field",
		},
	}

	validInputEvents := []testEvent{
		{
			Key:   "KeyOne",
			Value: "ValueOne",
		},
		{
			Key:   "KeyTwo",
			Value: "ValueTwo",
		},
		{
			Key:   "KeyThree",
			Value: "ValueThree",
		},
	}

	for _, event := range validInputEvents {
		eventData, err := json.Marshal(event)
		require.NoError(t, err)

		inputEvents = append(inputEvents, map[string]any{
			"data": string(eventData),
		})
	}

	testinfra := itest.SetupIntegrationTest(t, itest.Redis)
	defer testinfra.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, event := range inputEvents {
		xArgs := &redis.XAddArgs{
			Stream: "stream-key",
			Values: event,
		}
		err := testinfra.Redis.Cache.XAdd(ctx, xArgs).Err()
		require.NoError(t, err)
	}

	h := testEventHandler{
		cancel:         cancel,
		wantEventCount: len(validInputEvents),
	}

	consumer := RedisConsumer{
		ctx:   ctx,
		redis: testinfra.Redis.Cache,

		concurrency:   8,
		streamKey:     "stream-key",
		consumerGroup: "consumer-group",

		fn: h.handleEvent,
	}
	consumer.Consume()

	assert.ElementsMatch(t, validInputEvents, h.outputEvents)
}
