package data

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/app/util"
	"log/slog"
	"time"
)

const ActiveUserExpireFinished = 2 * time.Minute

func GetActiveCount(ctx context.Context, rdb *redis.Pool) (int, error) {
	return GetActiveCountWithExpiry(ctx, rdb, time.Now().Add(-ActiveUserExpireFinished).UnixMilli())
}

func GetActiveCountWithExpiry(ctx context.Context, rdb *redis.Pool, expireBefore int64) (int, error) {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	count, err := redis.Int64(conn.Do("ZREMRANGEBYSCORE", ActiveUsersZSet, "-inf", expireBefore))
	if err != nil {
		slog.Error("failed to expire users", "expireBefore", expireBefore, "trace", ctx.Value(util.TraceKey))
		return 0, err
	}
	if count > 0 {
		slog.Info("expired users with keys", "count", count)
	}
	count, err = redis.Int64(conn.Do("ZCARD", ActiveUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select users count: %w", err)
	}

	slog.Info("selected users count", "count", count, "trace", trace)
	return int(count), err
}

func AddActiveUser(ctx context.Context, rdb *redis.Pool, id string) error {
	return AddActiveUserOn(ctx, rdb, id, time.Now())
}

func AddActiveUserOn(ctx context.Context, rdb *redis.Pool, id string, expire time.Time) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", ActiveUsersZSet, "NX", float64(expire.UnixMilli()), id); err != nil {
		slog.Error("failed to add user", "id", id, "trace", trace, "err", err)
		return err
	}

	slog.Info("added user", "id", id, "trace", trace)
	return nil
}

func RemoveActiveUser(ctx context.Context, rdb *redis.Pool, id string) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", ActiveUsersZSet, id); err != nil {
		slog.Info("failed to remove user", "err", err, "trace", trace)
	}

	slog.Info("removed user", "id", id, "trace", trace)
	return nil
}
