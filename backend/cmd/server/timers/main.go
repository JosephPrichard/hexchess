package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/service/gamestate"
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

	tryExpireTimers(redisClient)

	slog.Info("finished initializing timer service", "timeTaken", time.Since(startTime).String())
}

// tryExpireTimers is a standalone function to expire timers for all game time zSets and ship them to job queues
// it runs inside its own single instance application because there is no real point in running this operation in parallel (it is very inexpensive)
func tryExpireTimers(redis cache.Redis) {
	partitions := model.GameIDPartitions()

	for _, partition := range partitions {
		service := gamestate.NewChessTimerService(redis, partition)
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()

			for range t.C {
				if _, err := service.TryExpireTimers(context.Background(), time.Now()); err != nil {
					slog.Error("failed to try expiring timers", "error", err)
				}
			}
		}()
	}
}
