package producers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

func ProduceFinishGame(ctx context.Context, xadder RedisXAdder, event model.FinishGameEvent) error {
	bytes, err := sonic.Marshal(event)
	if err != nil {
		return serrors.New("marshal finish game event", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: cache.FmtGameStreamKey(cache.Constants.FinishGameStreamKey, event.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd finished game event", err, "finishedGame", event)
	}

	slog.InfoContext(ctx, "produced finished game event", "msgID", msgID, "gameID", event.GameID, "streamKey", xArgs.Stream)
	return nil
}

func ProduceUpdtGameMetadata(ctx context.Context, xadder RedisXAdder, event model.UpdtGameMetadataEvent) error {
	bytes, err := sonic.Marshal(event)
	if err != nil {
		return serrors.New("marshal update game event", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: cache.FmtGameStreamKey(cache.Constants.UpdtGameMetaStreamKey, event.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd update game event", err, "gameUpdt", event)
	}

	slog.InfoContext(ctx, "produced update game event", "msgID", msgID, "gameUpdt", event, "streamKey", xArgs.Stream)
	return nil
}
