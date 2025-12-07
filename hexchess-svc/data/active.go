package data

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gomodule/redigo/redis"
)

const ActiveUsersZSet = "active_users"
const ActiveUserExpireFinished = time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

func GetActiveCount(ctx context.Context, conn redis.Conn, activeUsersZSet string) (int64, error) {
	return GetActiveCountWithExpiry(ctx, conn, activeUsersZSet, time.Now().Add(-ActiveUserExpireFinished).UnixMilli())
}

func GetActiveCountWithExpiry(ctx context.Context, conn redis.Conn, activeUsersZSet string, expireBefore int64) (int64, error) {
	count, err := redis.Int64(conn.Do("ZREMRANGEBYSCORE", activeUsersZSet, "-inf", expireBefore))
	if err != nil {
		return 0, fmt.Errorf("failed to 'ZREMRANGEBYSCORE' expire active users: %w", err)
	}
	if count > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", count)
	}
	count, err = redis.Int64(conn.Do("ZCARD", activeUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to 'ZCARD' active users count: %w", err)
	}
	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, err
}

func RetainActiveUser(ctx context.Context, rdb *Redis, id string) error {
	conn := rdb.Cache.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", rdb.ActiveUsersZSet, "XX", float64(time.Now().UnixMilli()), id); err != nil {
		return fmt.Errorf("failed to 'ZADD' active user %s: %w", id, err)
	}
	slog.InfoContext(ctx, "retained active user", "id", id)
	return nil
}

func AddActiveUser(ctx context.Context, rdb *Redis, id string) (int64, error) {
	now := time.Now()
	return AddActiveUserOn(ctx, rdb, id, now, now.Add(-ActiveUserExpireFinished).UnixMilli())
}

func AddActiveUserOn(ctx context.Context, rdb *Redis, id string, expiringOn time.Time, expireBefore int64) (int64, error) {
	conn := rdb.Cache.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", rdb.ActiveUsersZSet, "NX", float64(expiringOn.UnixMilli()), id); err != nil {
		return 0, fmt.Errorf("failed to 'ZADD' active user: %v: %w", id, err)
	}
	slog.InfoContext(ctx, "added active user", "id", id)
	return GetActiveCountWithExpiry(ctx, conn, rdb.ActiveUsersZSet, expireBefore)
}

func RemoveActiveUser(ctx context.Context, rdb *Redis, id string) (int64, error) {
	conn := rdb.Cache.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", rdb.ActiveUsersZSet, id); err != nil {
		return 0, fmt.Errorf("failed to 'ZREM' active user: %v: %w", id, err)
	}
	slog.InfoContext(ctx, "removed active user", "id", id)
	return GetActiveCount(ctx, conn, rdb.ActiveUsersZSet)
}
