package svc

import (
	"context"
	"fmt"
	"hexchess-svc/infra"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const ActiveUserExpireFinished = time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

func GetActiveCount(ctx context.Context, rdb *infra.Redis, activeUsersZSet string) (int64, error) {
	return GetActiveCountWithExpiry(ctx, rdb, activeUsersZSet, time.Now().Add(-ActiveUserExpireFinished).UnixMilli())
}

func GetActiveCountWithExpiry(ctx context.Context, rdb *infra.Redis, activeUsersZSet string, expireBefore int64) (int64, error) {
	removed, err := rdb.Cache.ZRemRangeByScore(ctx, activeUsersZSet, "-inf", fmt.Sprintf("%d", expireBefore)).Result()
	if err != nil {
		return 0, fmt.Errorf("get expired active users by range: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", removed)
	}
	count, err := rdb.Cache.ZCard(ctx, activeUsersZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count active users: %w", err)
	}
	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, nil
}

func RetainActiveUser(ctx context.Context, rdb *infra.Redis, id string) error {
	now := float64(time.Now().UnixMilli())
	res, err := rdb.Cache.ZAddXX(ctx, rdb.ActiveUsersZSet, redis.Z{Score: now, Member: id}).Result()
	if err != nil {
		return fmt.Errorf("add active user %s: %w", id, err)
	}
	if res > 0 {
		slog.InfoContext(ctx, "retained active user", "id", id)
	}
	return nil
}

func AddActiveUser(ctx context.Context, rdb *infra.Redis, id string) (int64, error) {
	now := time.Now()
	return AddActiveUserOn(ctx, rdb, id, now, now.Add(-ActiveUserExpireFinished).UnixMilli())
}

func AddActiveUserOn(ctx context.Context, rdb *infra.Redis, id string, expiringOn time.Time, expireBefore int64) (int64, error) {
	_, err := rdb.Cache.ZAddNX(ctx, rdb.ActiveUsersZSet, redis.Z{Score: float64(expiringOn.UnixMilli()), Member: id}).Result()
	if err != nil {
		return 0, fmt.Errorf("add active user %v: %w", id, err)
	}
	slog.InfoContext(ctx, "added active user", "id", id)

	return GetActiveCountWithExpiry(ctx, rdb, rdb.ActiveUsersZSet, expireBefore)
}

func RemoveActiveUser(ctx context.Context, rdb *infra.Redis, id string) (int64, error) {
	_, err := rdb.Cache.ZRem(ctx, rdb.ActiveUsersZSet, id).Result()
	if err != nil {
		return 0, fmt.Errorf("remove active user %v: %w", id, err)
	}
	slog.InfoContext(ctx, "removed active user", "id", id)

	return GetActiveCount(ctx, rdb, rdb.ActiveUsersZSet)
}
