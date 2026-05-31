package producers

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"log/slog"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

type RedisPublisher struct {
	redis db.RedisNames
}

func MakePublisher(redis db.Redis) RedisPublisher {
	return RedisPublisher{redis: redis.RedisNames}
}

func (p *RedisPublisher) PublishFinishGameEvent(ctx context.Context, xadder RedisXAdder, finishedGame model.FinishedGame) error {
	bytes, err := model.MarshalFinishedGame(finishedGame)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: p.redis.FinishGameStreamKey,
		Values: map[string]any{"payload": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event %+v: %w", finishedGame, err)
	}

	slog.InfoContext(ctx, "published finished game event", "msgID", msgID, "gameID", finishedGame.GameID)
	return nil
}

func (p *RedisPublisher) PublishUpdtGameEvent(ctx context.Context, xadder RedisXAdder, gameUpdt model.GameMetadataUpdt) error {
	bytes, err := model.MarshalGameMetadataUpdt(gameUpdt)
	if err != nil {
		return fmt.Errorf("marshal update game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: p.redis.UpdtGameMetaStreamKey,
		Values: map[string]any{"payload": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd update game event %+v: %w", gameUpdt, err)
	}

	slog.InfoContext(ctx, "published update game event", "msgID", msgID, "gameUpdt", gameUpdt)
	return nil
}
