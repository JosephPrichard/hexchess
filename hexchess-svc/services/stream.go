package svc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func StartStreamReaders(ctx context.Context, svc *Services) {
	go MakeFinishGameStreamer(ctx, svc).EventLoop()
}

func MakeFinishGameStreamer(ctx context.Context, svc *Services) *RedisStreamer[FinishGameEvent] {
	return &RedisStreamer[FinishGameEvent]{
		Context: ctx,
		Client:  svc.Redis.GameStore,

		Concurrency:   8,
		StreamKey:     svc.Redis.FinishGameStreamKey,
		ConsumerGroup: FinishGameConsumerGroup,

		HandleEvent:    svc.insertFinishedGameEvent,
		UnmarshalEvent: UnmarshalFinishGameEvent,
	}
}

type RedisStreamer[Event any] struct {
	Context context.Context
	Client  *redis.Client

	Concurrency   int64
	StreamKey     string
	ConsumerGroup string

	HandleEvent    func(ctx context.Context, event Event) error
	UnmarshalEvent func([]byte) (Event, error)

	waitGroup sync.WaitGroup
}

type AckSignal int

const (
	SendAck AckSignal = iota
	DontSendAck
)

func (stream *RedisStreamer[Event]) handleXReadMessage(msg redis.XMessage) {
	defer stream.waitGroup.Done()

	ackSignal := func() AckSignal {
		// send the ack if data is invalid OR message succeeds, retry otherwise
		data, ok := msg.Values["data"].(string)
		if !ok {
			slog.Error("failed to read message, does not contain 'data' field")
			return SendAck
		}

		event, err := stream.UnmarshalEvent([]byte(data))
		if err != nil {
			slog.Error("failed to unmarshal event", "err", err)
			return SendAck
		}

		if err := stream.HandleEvent(stream.Context, event); err != nil {
			slog.Error("failed to handle event", "err", err)
			return DontSendAck
		}
		return SendAck
	}()

	if ackSignal == DontSendAck {
		return
	}
	if err := stream.Client.XAck(stream.Context, stream.StreamKey, stream.ConsumerGroup, msg.ID).Err(); err != nil {
		slog.Error("failed to acknowledge event", "id", msg.ID, "err", err)
	} else {
		slog.Info("acknowledged finished event", "id", msg.ID)
	}
}

func (stream *RedisStreamer[Event]) EventLoop() error {
	consumerName := uuid.NewString()
	streamKey := stream.StreamKey

	err := stream.Client.XGroupCreateMkStream(stream.Context, streamKey, stream.ConsumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create games stream consumer group: %w", err)
	}

	slog.Info("created finish game event streamer", "consumerName", consumerName)

	for {
		entries, err := stream.Client.XReadGroup(stream.Context, &redis.XReadGroupArgs{
			Group:    stream.ConsumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamKey, ">"}, // ">" means only undelivered messages
			Count:    stream.Concurrency,
		}).Result()

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
				stream.waitGroup.Add(1)
				go stream.handleXReadMessage(msg)
			}
		}
		stream.waitGroup.Wait()
	}
}
