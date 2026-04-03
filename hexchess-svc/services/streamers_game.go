package svc

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
)

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

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

func (svc *Services) pushFinishGameEvent(ctx context.Context, xadder RedisXAdder, event FinishGameEvent) error {
	streamKey := svc.Redis.FinishGameStreamKey

	bytes, err := MarshalFinishGameEvent(event)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event: %w", err)
	}

	slog.InfoContext(ctx, "pushed finished game event", "id", msgID, "gameID", event.GameID)
	return nil
}
