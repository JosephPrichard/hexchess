package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/trigger"
	"hexchess-svc/utils/config"
	"log/slog"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	startTime := time.Now()

	cfg := config.Load()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	trigger.NewTimerTrigger(redisClient, time.Second).Start()

	slog.Info("finished initializing trigger service", "timeTaken", time.Since(startTime).String())
}
