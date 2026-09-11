package trigger

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/service"
	"log/slog"
	"time"
)

type TimerTrigger struct {
	redis      cache.Redis
	period     time.Duration
	partitions []string
}

func NewTimerTrigger(redis cache.Redis, period time.Duration) *TimerTrigger {
	return &TimerTrigger{redis: redis, period: period, partitions: model.GameIDPartitions()}
}

func (trigger *TimerTrigger) Start() {
	for _, partition := range trigger.partitions {
		go trigger.loop(partition)
	}

	slog.Info("started timer triggers", "partition", trigger.partitions)
}

func (trigger *TimerTrigger) loop(partition string) {
	service := service.NewChessTimerService(trigger.redis, partition)

	t := time.NewTicker(trigger.period)
	defer t.Stop()

	for range t.C {
		trigger.iteration(service)
	}
}

func (trigger *TimerTrigger) iteration(service *service.TimerService) {
	ctx, cancel := context.WithTimeout(context.Background(), trigger.period*2)
	defer cancel()

	if _, err := service.TryExpireTimers(ctx, time.Now()); err != nil {
		slog.Error("failed to try expiring timers", "error", err)
	}
}
