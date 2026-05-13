package queue

import (
	"context"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/internal/errutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func StartRedisQueueConsumers(ctx context.Context, services svc.HexchessAPI, redis db.Redis) {
	h := EventHandler{Services: services}

	handlerList := []RedisQueueHandler{
		&RedisConsumer[model.FinishedGame]{
			Context: ctx,
			Client:  redis.GameStore,

			Concurrency:   8,
			StreamKey:     redis.FinishGameStreamKey,
			ConsumerGroup: FinishGameConsumerGroup,

			HandleEvent: h.HandleFinishedGameEvent,
		},
	}

	for _, handler := range handlerList {
		go handler.EventLoop()
		slog.InfoContext(ctx, "started redis stream consumer for handler", "handler", fmt.Sprintf("%+v", handler))
	}
}

const FinishGameConsumerGroup = "finish_game:consumer"

type RedisQueueHandler interface {
	EventLoop() error
}

type RedisConsumer[Event any] struct {
	Context context.Context
	Client  *redis.Client

	ConsumerName  string
	Concurrency   int64
	StreamKey     string
	ConsumerGroup string

	HandleEvent func(ctx context.Context, eventData string) error

	waitGroup sync.WaitGroup
}

func (stream *RedisConsumer[Event]) EventLoop() error {
	consumerID := uuid.NewString()
	streamKey := stream.StreamKey

	err := stream.Client.XGroupCreateMkStream(stream.Context, streamKey, stream.ConsumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create games stream consumer group: %w", err)
	}

	for {
		slog.Info("begin redis stream consumer read operation", "consumerID", consumerID, "streamKey", streamKey)

		xArgs := &redis.XReadGroupArgs{
			Group:    stream.ConsumerGroup,
			Consumer: consumerID,
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
			slog.Error("failed to read from finish redis stream", "error", err)
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
		anyData := msg.Values["data"]
		data, ok := anyData.(string)
		if !ok {
			slog.Error("failed to read message, 'data' field is incorrect type", "type", fmt.Sprintf("%T", anyData))
			return sendAck
		}

		err := stream.HandleEvent(stream.Context, data)

		if err != nil {
			slog.Error("failed to handle event", "error", err)
			if errutil.IsType[NonRetryableQueueError](err) {
				return sendAck
			} else {
				return dontSendAck
			}
		}

		return sendAck
	}()

	if ack == dontSendAck {
		return
	}
	if err := stream.Client.XAck(stream.Context, stream.StreamKey, stream.ConsumerGroup, msg.ID).Err(); err != nil {
		slog.Error("failed to acknowledge event", "id", msg.ID, "error", err)
	} else {
		slog.Info("acknowledged finished event", "id", msg.ID)
	}
}
