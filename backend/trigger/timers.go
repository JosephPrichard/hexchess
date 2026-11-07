package trigger

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/game"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"time"
)

type TimerTrigger struct {
	ctx        context.Context
	redis      cache.Redis
	period     time.Duration
	partitions []string
}

func NewTimerTrigger(ctx context.Context, redis cache.Redis, period time.Duration) *TimerTrigger {
	return &TimerTrigger{ctx: ctx, redis: redis, period: period, partitions: gameid.GameIDPartitions()}
}

func (trigger *TimerTrigger) Start() {
	for _, partition := range trigger.partitions {
		go trigger.outerLoop(partition)
	}

	slog.Info("started timer triggers", "partition", trigger.partitions)
}

func (trigger *TimerTrigger) outerLoop(partition string) {
	// guarantee that this continues to execute for the rest of the process, even if the inner loop function returns/errors
	t := time.NewTicker(trigger.period)
	defer t.Stop()

	for range t.C {
		slog.Info("begin timer timer trigger loop", "partition", partition)
		trigger.innerLoop(partition)
	}
}

func (trigger *TimerTrigger) innerLoop(partition string) {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("error in timer trigger loop", "error", err)
		}
	}()

	timerService := game.NewChessTimerService(trigger.redis, partition)

	t := time.NewTicker(trigger.period)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			trigger.iteration(timerService)
		case <-trigger.ctx.Done():
			err := trigger.ctx.Err()
			slog.Log(trigger.ctx, slogutil.ErrorLevel(err), "exiting timer trigger loop", "error", err)
			return
		}
	}
}

func (trigger *TimerTrigger) iteration(service *game.TimerService) {
	ctx, cancel := context.WithTimeout(context.Background(), trigger.period*2)
	defer cancel()

	if _, err := service.TryExpireTimers(ctx, time.Now()); err != nil {
		slog.Error("failed to try expiring timers", "error", err)
	}
}
