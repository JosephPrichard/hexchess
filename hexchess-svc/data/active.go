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
const ActiveUserExpireFinished = 2 * time.Hour // the caller should manually remove, but this is a stopgap in case

func GetActiveCount(ctx context.Context, conn redis.Conn, activeUsersZSet string) (int64, error) {
	return GetActiveCountWithExpiry(ctx, conn, activeUsersZSet, time.Now().Add(-ActiveUserExpireFinished).UnixMilli())
}

func GetActiveCountWithExpiry(ctx context.Context, conn redis.Conn, activeUsersZSet string, expireBefore int64) (int64, error) {
	count, err := redis.Int64(conn.Do("ZREMRANGEBYSCORE", activeUsersZSet, "-inf", expireBefore))
	if err != nil {
		slog.ErrorContext(ctx, "failed to expire users", "expireBefore", expireBefore, "trace", ctx.Value(lib.TraceKey))
		return 0, err
	}
	if count > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", count)
	}
	count, err = redis.Int64(conn.Do("ZCARD", activeUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select users count: %w", err)
	}
	slog.InfoContext(ctx, "selected users count", "count", count)
	return count, err
}

func RetainActiveUser(ctx context.Context, rdb Rdb, id string) error {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", rdb.ActiveUsersZSet, "NX", float64(time.Now().UnixMilli()), id); err != nil {
		slog.ErrorContext(ctx, "failed to add user", "id", id, "err", err)
		return err
	}
	slog.InfoContext(ctx, "added active user", "id", id)
	return nil
}

func AddActiveUser(ctx context.Context, rdb Rdb, id string) (int64, error) {
	now := time.Now()
	return AddActiveUserOn(ctx, rdb, id, now, now.Add(-ActiveUserExpireFinished).UnixMilli())
}

func AddActiveUserOn(ctx context.Context, rdb Rdb, id string, expire time.Time, expireBefore int64) (int64, error) {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", rdb.ActiveUsersZSet, "NX", float64(expire.UnixMilli()), id); err != nil {
		slog.ErrorContext(ctx, "failed to add user", "id", id, "err", err)
		return 0, err
	}
	slog.InfoContext(ctx, "added active user", "id", id)
	return GetActiveCountWithExpiry(ctx, conn, rdb.ActiveUsersZSet, expireBefore)
}

func RemoveActiveUser(ctx context.Context, rdb Rdb, id string) (int64, error) {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", rdb.ActiveUsersZSet, id); err != nil {
		slog.InfoContext(ctx, "failed to remove user", "err", err)
	}
	slog.InfoContext(ctx, "removed active user", "id", id)
	return GetActiveCount(ctx, conn, rdb.ActiveUsersZSet)
}
