package events

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"
	"sync"
	"testing"
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

func (h *testEventHandler) handleEvent(_ context.Context, event testEvent) error {
	h.lock.Lock()
	h.outputEvents = append(h.outputEvents, event)
	h.lock.Unlock()

	if len(h.outputEvents) == h.wantEventCount {
		go h.cancel()
	}
	return nil
}

func (h *testEventHandler) unmarshalEvent(bytes []byte) (testEvent, error) {
	var e testEvent
	err := json.Unmarshal(bytes, &e)
	return e, err
}

func TestGameFinishStreamer(t *testing.T) {
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

	testinfra := itest.SetupTestInfra(t, itest.Redis)
	defer testinfra.Close()

	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), logutil.Trace, t.Name()))
	defer cancel()

	for _, event := range inputEvents {
		xArgs := &redis.XAddArgs{
			Stream: "stream-key",
			Values: event,
		}
		err := testinfra.Redis.Cache.XAdd(ctx, xArgs).Err()
		require.NoError(t, err)
	}

	handler := testEventHandler{
		cancel:         cancel,
		wantEventCount: len(validInputEvents),
	}

	consumer := RedisConsumer[testEvent]{
		Context: ctx,
		Client:  testinfra.Redis.Cache,

		Concurrency:   8,
		StreamKey:     "stream-key",
		ConsumerGroup: "consumer-group",

		HandleEvent:    handler.handleEvent,
		UnmarshalEvent: handler.unmarshalEvent,
	}
	consumer.EventLoop()

	assert.ElementsMatch(t, validInputEvents, handler.outputEvents)
}
