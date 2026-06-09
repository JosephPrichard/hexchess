package producers

import (
	"context"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/db"
	"hexchess-svc/lib/serrors"
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
		return serrors.New("marshal finish game event", err)
	}
	xArgs := &redis.XAddArgs{
		Stream: p.redis.FinishGameStreamKey,
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("marshal finished game event", err, "finishedGame", finishedGame)
	}
	slog.InfoContext(ctx, "published finished game event", "msgID", msgID, "gameID", finishedGame.GameID, "streamKey", p.redis.FinishGameStreamKey)
	return nil
}

func (p *RedisPublisher) PublishUpdtGameEvent(ctx context.Context, xadder RedisXAdder, gameUpdt model.GameMetadataUpdt) error {
	bytes, err := model.MarshalGameMetadataUpdt(gameUpdt)
	if err != nil {
		return serrors.New("marshal update game event", err)
	}
	xArgs := &redis.XAddArgs{
		Stream: p.redis.UpdtGameMetaStreamKey,
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd update game event", err, "gameUpdt", gameUpdt)
	}
	slog.InfoContext(ctx, "published update game event", "msgID", msgID, "gameUpdt", gameUpdt, "streamKey", p.redis.UpdtGameMetaStreamKey)
	return nil
}
