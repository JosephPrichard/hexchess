package svc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const ActiveUserMaxage = 5 * time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

func (svc *HexchessServices) IsActiveUser(ctx context.Context, id string) bool {
	_, err := svc.redis.Cache.ZScore(ctx, svc.redis.ActiveUsersZSet, id).Result()
	return err == nil
}

func (svc *HexchessServices) GetActiveCount(ctx context.Context) (int64, error) {
	expireBefore := svc.entropy.GetTime().Add(-ActiveUserMaxage).UnixMilli()

	removed, err := svc.redis.Cache.ZRemRangeByScore(ctx, svc.redis.ActiveUsersZSet, "-inf", fmt.Sprintf("%d", expireBefore)).Result()
	if err != nil {
		return 0, fmt.Errorf("get expired active users by range: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", removed, "expireBefore", expireBefore)
	}
	count, err := svc.redis.Cache.ZCard(ctx, svc.redis.ActiveUsersZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count active users: %w", err)
	}
	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, nil
}

func (svc *HexchessServices) RetainActiveUser(ctx context.Context, id string) error {
	updtTime := float64(svc.entropy.GetTime().UnixMilli())

	_, err := svc.redis.Cache.ZAddXX(ctx, svc.redis.ActiveUsersZSet, redis.Z{Score: updtTime, Member: id}).Result()
	if err != nil {
		return fmt.Errorf("retain active user %s: %w", id, err)
	}
	slog.InfoContext(ctx, "retained active user", "id", id)
	return nil
}

func (svc *HexchessServices) AddActiveUser(ctx context.Context, id string) (int64, error) {
	updtTime := float64(svc.entropy.GetTime().UnixMilli())

	_, err := svc.redis.Cache.ZAddNX(ctx, svc.redis.ActiveUsersZSet, redis.Z{Score: updtTime, Member: id}).Result()
	if err != nil {
		return 0, fmt.Errorf("add active user %v: %w", id, err)
	}
	slog.InfoContext(ctx, "added active user", "id", id)

	return svc.GetActiveCount(ctx)
}

func (svc *HexchessServices) RemoveActiveUser(ctx context.Context, id string) (int64, error) {
	res, err := svc.redis.Cache.ZRem(ctx, svc.redis.ActiveUsersZSet, id).Result()
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
