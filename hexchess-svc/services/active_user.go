package svc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const ActiveUserMaxage = 5 * time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

func (svc *Services) GetActiveCount(ctx context.Context) (int64, error) {
	expireBefore := svc.EntropySource.GetNow().Add(-ActiveUserMaxage).UnixMilli()

	removed, err := svc.Redis.Cache.ZRemRangeByScore(ctx, svc.Redis.ActiveUsersZSet, "-inf", fmt.Sprintf("%d", expireBefore)).Result()
	if err != nil {
		return 0, fmt.Errorf("get expired active users by range: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", removed, "expireBefore", expireBefore)
	}
	count, err := svc.Redis.Cache.ZCard(ctx, svc.Redis.ActiveUsersZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count active users: %w", err)
	}
	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, nil
}

func (svc *Services) RetainActiveUser(ctx context.Context, id string) error {
	now := svc.EntropySource.GetNow()

	_, err := svc.Redis.Cache.ZAddXX(ctx, svc.Redis.ActiveUsersZSet, redis.Z{Score: float64(now.UnixMilli()), Member: id}).Result()
	if err != nil {
		return fmt.Errorf("retain active user %s: %w", id, err)
	}
	slog.InfoContext(ctx, "retained active user", "id", id)
	return nil
}

func (svc *Services) AddActiveUser(ctx context.Context, id string) (int64, error) {
	now := svc.EntropySource.GetNow()

	_, err := svc.Redis.Cache.ZAddNX(ctx, svc.Redis.ActiveUsersZSet, redis.Z{Score: float64(now.UnixMilli()), Member: id}).Result()
	if err != nil {
		return 0, fmt.Errorf("add active user %v: %w", id, err)
	}
	slog.InfoContext(ctx, "added active user", "id", id)

	return svc.GetActiveCount(ctx)
}

func (svc *Services) RemoveActiveUser(ctx context.Context, id string) (int64, error) {
	res, err := svc.Redis.Cache.ZRem(ctx, svc.Redis.ActiveUsersZSet, id).Result()
	if err != nil {
		return 0, fmt.Errorf("remove active user %v: %w", id, err)
	}
	if res > 0 {
		slog.InfoContext(ctx, "removed active user", "id", id)
	} else {
		slog.WarnContext(ctx, "did not remove active user", "id", id)
	}

	return svc.GetActiveCount(ctx)
}
