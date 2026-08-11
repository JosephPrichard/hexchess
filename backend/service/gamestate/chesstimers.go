package gamestate

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"strconv"
	"time"
	"unicode/utf8"
)

type TimerService struct {
	redis cache.Redis

	zDequeueXAddKeys []string
}

func NewChessTimerService(redis cache.Redis, partitionKey string) *TimerService {
	p, _ := utf8.DecodeRuneInString(partitionKey)
	return &TimerService{
		redis:            redis,
		zDequeueXAddKeys: []string{cache.FmtGameTimersZSet(p), cache.FmtStreamKey(cache.Constants.FinishGameStreamKey, partitionKey)},
	}
}

func (services *TimerService) TryExpireTimers(ctx context.Context, expireBeforeTime time.Time) (any, error) {
	defer perf.New().Log()

	expireBefore := strconv.Itoa(int(expireBeforeTime.UnixMilli()))

	cmd := cache.ZDequeueXAdd.Run(ctx, services.redis.PrimaryClient, services.zDequeueXAddKeys, expireBefore)
	keys, err := cmd.Result()
	if err != nil {
		return nil, serrors.New("executing zdequeue xadd script", err,
			"expireBefore", expireBefore, "zDequeueXAddKeys", services.zDequeueXAddKeys)
	}

	slog.InfoContext(ctx, "expired timers from sorted set",
		"dequeuedKeys", keys, "expireBefore", expireBefore, "zDequeueXAddKeys", services.zDequeueXAddKeys)
	return keys, nil
}
