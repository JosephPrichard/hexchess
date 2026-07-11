package producers

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"hexchess-svc/queue"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

type RedisPublisher struct {
	redis db.RedisNames
}

func NewPublisher(redis db.Redis) RedisPublisher {
	return RedisPublisher{redis: redis.RedisNames}
}

func (p *RedisPublisher) PublishFinishGameEvent(ctx context.Context, xadder RedisXAdder, finishedGame model.FinishedGame) error {
	bytes, err := model.MarshalFinishedGame(finishedGame)
	if err != nil {
		return serrors.New("marshal finish game event", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: queue.FmtGameStreamKey(p.redis.FinishGameStreamKey, finishedGame.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("marshal finished game event", err, "finishedGame", finishedGame)
	}

	slog.InfoContext(ctx, "published finished game event", "msgID", msgID, "gameID", finishedGame.GameID, "streamKey", xArgs.Stream)
	return nil
}

func (p *RedisPublisher) PublishUpdtGameEvent(ctx context.Context, xadder RedisXAdder, gameUpdt model.GameMetadataUpdt) error {
	bytes, err := model.MarshalGameMetadataUpdt(gameUpdt)
	if err != nil {
		return serrors.New("marshal update game event", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: queue.FmtGameStreamKey(p.redis.UpdtGameMetaStreamKey, gameUpdt.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd update game event", err, "gameUpdt", gameUpdt)
	}

	slog.InfoContext(ctx, "published update game event", "msgID", msgID, "gameUpdt", gameUpdt, "streamKey", xArgs.Stream)
	return nil
}
