package dal

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/app/util"
	"log/slog"
	"time"
)

func GetUsersCount(ctx context.Context, rdb *redis.Pool) (int64, error) {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if err := ExpireCountedUsers(ctx, conn); err != nil {
		return 0, err
	}
	count, err := redis.Int64(conn.Do("ZCARD", ActiveUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select users count: %w", err)
	}

	slog.Info("selected users count", "count", count, "trace", trace)
	return count, err
}

func AddCountedUser(ctx context.Context, rdb *redis.Pool, id string) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", ActiveUsersZSet, "NX", "CH", float64(time.Now().UnixMilli()), id); err != nil {
		slog.Error("failed to add user", "id", id, "trace", trace, "err", err)
		return err
	}

	slog.Info("added user", "id", id, "trace", trace)
	return nil
}

func RemoveCountedUser(ctx context.Context, rdb *redis.Pool, id string) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", ActiveUsersZSet, id); err != nil {
		slog.Info("failed to remove user", "err", err, "trace", trace)
	}

	slog.Info("removed user", "id", id, "trace", trace)
	return nil
}

func GetChessStateCount(ctx context.Context, rdb *redis.Pool) (int64, error) {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, GamesZSet); err != nil {
		return 0, err
	}
	count, err := redis.Int64(conn.Do("ZCARD", GamesZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select chess count: %w", err)
	}

	slog.Info("selected chess count", "count", count, "trace", trace)
	return count, err
}

const UserExpireFinished = 2 * time.Minute

func ExpireCountedUsers(ctx context.Context, conn redis.Conn) error {
	return ExpireCountedUsersBefore(ctx, conn, time.Now().Add(-UserExpireFinished).UnixMilli())
}

func ExpireCountedUsersBefore(ctx context.Context, conn redis.Conn, expireBefore int64) error {
	keys, err := redis.Strings(conn.Do("ZREMRANGEBYSCORE", ActiveUsersZSet, "-inf", expireBefore))
	if err != nil {
		slog.Error("failed to expire users", "expireBefore", expireBefore, "trace", ctx.Value(util.TraceKey))
		return err
	}
	if len(keys) > 0 {
		slog.Info("expired users with keys", "keys", keys)
	}
	return nil
}
