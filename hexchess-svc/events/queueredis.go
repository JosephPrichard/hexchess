package events

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/db"
	svc "hexchess-svc/service"
	"log/slog"
	"sync"
)

func StartRedisQueueConsumers(ctx context.Context, services svc.HexchessAPI, redis db.Redis) {
	handlerList := []RedisQueueHandler{
		&RedisConsumer[svc.FinishGameEvent]{
			Context: ctx,
			Client:  redis.GameStore,

			Concurrency:   8,
			StreamKey:     redis.FinishGameStreamKey,
			ConsumerGroup: FinishGameConsumerGroup,

			HandleEvent:    services.InsertFinishedGameEvent,
			UnmarshalEvent: svc.UnmarshalFinishGameEvent,
		},
	}

	for _, handler := range handlerList {
		go handler.EventLoop()
	}
}

const FinishGameConsumerGroup = "finish_game:consumer"

type RedisQueueHandler interface {
	EventLoop() error
}

type RedisConsumer[Event any] struct {
	Context context.Context
	Client  *redis.Client

	Concurrency   int64
	StreamKey     string
	ConsumerGroup string

	HandleEvent    func(ctx context.Context, event Event) error
	UnmarshalEvent func([]byte) (Event, error)

	waitGroup sync.WaitGroup
}

func (stream *RedisConsumer[Event]) EventLoop() error {
	consumerName := uuid.NewString()
	streamKey := stream.StreamKey

	err := stream.Client.XGroupCreateMkStream(stream.Context, streamKey, stream.ConsumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create games stream consumer group: %w", err)
	}

	slog.Info("created finish game event streamer", "consumerName", consumerName)

	for {
		xArgs := &redis.XReadGroupArgs{
			Group:    stream.ConsumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamKey, ">"}, // ">" means only undelivered messages
			Count:    stream.Concurrency,
		}

		entries, err := stream.Client.XReadGroup(stream.Context, xArgs).Result()
		switch err {
		case nil:
			// handle the event
		case context.Canceled:
			slog.Info("context cancelled, exiting finish event loop")
			return nil
		default:
			slog.Error("failed to read from finish redis stream", "err", err)
			continue
		}

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				stream.waitGroup.Go(func() {
					stream.handleXReadMessage(msg)
				})
			}
		}

		stream.waitGroup.Wait()
	}
}

type ackSignal int

const (
	sendAck ackSignal = iota
	dontSendAck
)

func (stream *RedisConsumer[Event]) handleXReadMessage(msg redis.XMessage) {
	ack := func() ackSignal {
		// send the ack if data is invalid OR message succeeds, retry otherwise
		data, ok := msg.Values["data"].(string)
		if !ok {
			slog.Error("failed to read message, does not contain 'data' field")
			return sendAck
		}

		event, err := stream.UnmarshalEvent([]byte(data))
		if err != nil {
			slog.Error("failed to unmarshal event", "err", err, "type", fmt.Sprintf("%T", event))
			return sendAck
		}

		if err := stream.HandleEvent(stream.Context, event); err != nil {
			slog.Error("failed to handle event", "err", err)
			return dontSendAck
		}
		return sendAck
	}()

	if ack == dontSendAck {
		return
	}
	if err := stream.Client.XAck(stream.Context, stream.StreamKey, stream.ConsumerGroup, msg.ID).Err(); err != nil {
		slog.Error("failed to acknowledge event", "id", msg.ID, "err", err)
	} else {
		slog.Info("acknowledged finished event", "id", msg.ID)
	}
}
