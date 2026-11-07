package trigger

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"time"
)

type TriggerConfig struct {
	RedisClient cache.Redis
	Period      time.Duration
}

func Execute(ctx context.Context, cfg TriggerConfig) {
	if ctx == nil {
		ctx = context.Background() // if this is set, the triggers can never be canceled
	}
	if cfg.Period == 0 {
		cfg.Period = time.Second
	}

	NewTimerTrigger(ctx, cfg.RedisClient, cfg.Period).Start()

	<-ctx.Done()
	err := ctx.Err()
	slog.Log(ctx, slogutil.ErrorLevel(err), "exiting trigger execution loop", "error", err)
}
