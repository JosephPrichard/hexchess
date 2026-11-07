package producers

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/cache"
	"hexchess-svc/queue"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

func ProduceFinishGame(ctx context.Context, xadder RedisXAdder, event queue.FinishGameJob) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return serrors.New("marshal finish game event", err)
	}
	jsonStr := string(bytes)

	xArgs := &redis.XAddArgs{
		Stream: cache.FmtGameStreamKey(cache.Constants.FinishGameStreamKey, event.GameID),
		Values: map[string]any{"data": jsonStr},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd finished game event", err, "payload", jsonStr)
	}

	slog.InfoContext(ctx, "produced finished game event", "msgID", msgID, "gameID", event.GameID, "streamKey", xArgs.Stream, "payload", jsonStr)
	return nil
}

func ProduceUpdtGameMetadata(ctx context.Context, xadder RedisXAdder, event queue.UpdtGameMetadataJob) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return serrors.New("marshal update game event", err)
	}
	jsonStr := string(bytes)

	xArgs := &redis.XAddArgs{
		Stream: cache.FmtGameStreamKey(cache.Constants.UpdtGameMetaStreamKey, event.GameID),
		Values: map[string]any{"data": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return serrors.New("xadd update game event", err, "payload", jsonStr)
	}

	slog.InfoContext(ctx, "produced update game event", "msgID", msgID, "streamKey", xArgs.Stream, "payload", jsonStr)
	return nil
}
