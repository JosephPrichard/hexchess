package data

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/lib"
	"log/slog"
	"time"
)

const ActiveUsersZSet = "active_users"
const ActiveUserExpireFinished = 2 * time.Minute

func GetActiveCount(ctx context.Context, rdb Rdb) (int, error) {
	return GetActiveCountWithExpiry(ctx, rdb, time.Now().Add(-ActiveUserExpireFinished).UnixMilli())
}

func GetActiveCountWithExpiry(ctx context.Context, rdb Rdb, expireBefore int64) (int, error) {
	conn := rdb.Get()
	defer conn.Close()

	count, err := redis.Int64(conn.Do("ZREMRANGEBYSCORE", rdb.ActiveUsersZSet, "-inf", expireBefore))
	if err != nil {
		slog.ErrorContext(ctx, "failed to expire users", "expireBefore", expireBefore, "trace", ctx.Value(lib.TraceKey))
		return 0, err
	}
	if count > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", count)
	}
	count, err = redis.Int64(conn.Do("ZCARD", rdb.ActiveUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select users count: %w", err)
	}
	slog.InfoContext(ctx, "selected users count", "count", count)
	return int(count), err
}

func AddActiveUser(ctx context.Context, rdb Rdb, id string) error {
	return AddActiveUserOn(ctx, rdb, id, time.Now())
}

func AddActiveUserOn(ctx context.Context, rdb Rdb, id string, expire time.Time) error {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", rdb.ActiveUsersZSet, "NX", float64(expire.UnixMilli()), id); err != nil {
		slog.ErrorContext(ctx, "failed to add user", "id", id, "err", err)
		return err
	}
	slog.InfoContext(ctx, "added user", "id", id)
	return nil
}

func RemoveActiveUser(ctx context.Context, rdb Rdb, id string) error {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", rdb.ActiveUsersZSet, id); err != nil {
		slog.InfoContext(ctx, "failed to remove user", "err", err)
	}
	slog.InfoContext(ctx, "removed user", "id", id)
	return nil
}
