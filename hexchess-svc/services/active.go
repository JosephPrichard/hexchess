package svc

import (
	"context"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type ActiveScenario struct {
	outbound.Generator
	maxAge time.Duration
}

func MakeActiveScenario() ActiveScenario {
	return ActiveScenario{&outbound.NDGenerator{}, ActiveUserMaxage}
}

const ActiveUserMaxage = time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

func (s ActiveScenario) GetActiveCount(ctx context.Context, rdb *db.Rdb) (int64, error) {
	expireBefore := s.GetNow().Add(-s.maxAge).UnixMilli()

	removed, err := rdb.Cache.ZRemRangeByScore(ctx, rdb.ActiveUsersZSet, "-inf", fmt.Sprintf("%d", expireBefore)).Result()
	if err != nil {
		return 0, fmt.Errorf("get expired active users by range: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", removed, "expireBefore", expireBefore)
	}
	count, err := rdb.Cache.ZCard(ctx, rdb.ActiveUsersZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count active users: %w", err)
	}
	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, nil
}

func (s ActiveScenario) RetainActiveUser(ctx context.Context, rdb *db.Rdb, id string) error {
	now := float64(s.GetNow().UnixMilli())

	_, err := rdb.Cache.ZAddXX(ctx, rdb.ActiveUsersZSet, redis.Z{Score: now, Member: id}).Result()
	if err != nil {
		return fmt.Errorf("retain active user %s: %w", id, err)
	}
	slog.InfoContext(ctx, "retained active user", "id", id, "score", now)
	return nil
}

func (s ActiveScenario) AddActiveUser(ctx context.Context, rdb *db.Rdb, id string) (int64, error) {
	now := float64(s.GetNow().UnixMilli())

	_, err := rdb.Cache.ZAddNX(ctx, rdb.ActiveUsersZSet, redis.Z{Score: now, Member: id}).Result()
	if err != nil {
		return 0, fmt.Errorf("add active user %v: %w", id, err)
	}
	slog.InfoContext(ctx, "added active user", "id", id, "score", now)

	return s.GetActiveCount(ctx, rdb)
}

func (s ActiveScenario) RemoveActiveUser(ctx context.Context, rdb *db.Rdb, id string) (int64, error) {
	res, err := rdb.Cache.ZRem(ctx, rdb.ActiveUsersZSet, id).Result()
	if err != nil {
		return 0, fmt.Errorf("remove active user %v: %w", id, err)
	}
	if res > 0 {
		slog.InfoContext(ctx, "removed active user", "id", id)
	} else {
		slog.WarnContext(ctx, "did not remove active user", "id", id)
	}

	return s.GetActiveCount(ctx, rdb)
}
