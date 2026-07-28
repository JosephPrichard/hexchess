package producers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/queue"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

type StreamProducer struct {
	redis cache.RedisNames
}

func NewPublisher(redis cache.Redis) StreamProducer {
	return StreamProducer{redis: redis.RedisNames}
}

func (p *StreamProducer) ProduceFinishGame(ctx context.Context, xadder RedisXAdder, finishedGame model.FinishedGame) error {
	bytes, err := sonic.Marshal(finishedGame)
	if err != nil {
		return serrors.New("marshal finish game event", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: queue.FmtGameStreamKey(p.redis.FinishGameStreamKey, finishedGame.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd finished game event", err, "finishedGame", finishedGame)
	}

	slog.InfoContext(ctx, "produced finished game event", "msgID", msgID, "gameID", finishedGame.GameID, "streamKey", xArgs.Stream)
	return nil
}

func (p *StreamProducer) ProduceUpdtGameMetadata(ctx context.Context, xadder RedisXAdder, gameUpdt model.GameMetadataUpdt) error {
	bytes, err := sonic.Marshal(gameUpdt)
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

	slog.InfoContext(ctx, "produced update game event", "msgID", msgID, "gameUpdt", gameUpdt, "streamKey", xArgs.Stream)
	return nil
}
