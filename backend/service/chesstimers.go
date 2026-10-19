package service

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/perf"
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

func (service *TimerService) TryExpireTimers(ctx context.Context, expireBeforeTime time.Time) (any, error) {
	defer perf.New().Log()

	expireBefore := strconv.Itoa(int(expireBeforeTime.UnixMilli()))

	cmd := cache.PollGameTimers.Run(ctx, service.redis.PrimaryClient, service.zDequeueXAddKeys, expireBefore)
	keys, err := cmd.Result()
	if err != nil {
		return nil, errutil.Wrap("executing poll game timers script", err,
			"expireBefore", expireBefore, "zDequeueXAddKeys", service.zDequeueXAddKeys)
	}

	slog.InfoContext(ctx, "expired timers from sorted set",
		"dequeuedKeys", keys, "expireBefore", expireBefore, "zDequeueXAddKeys", service.zDequeueXAddKeys)
	return keys, nil
}
