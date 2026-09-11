package service

import (
	"context"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const ActiveUserMaxAge = 5 * time.Minute // the caller should manually remove, but this is a stopgap in case the server is stopped before that is the case

type ActiveUserService struct {
	redis       cache.Redis
	broadcaster pubsub.Broadcaster
	entropy     entropy.Generator
}

func NewActiveUserService(redis cache.Redis, broadcaster pubsub.Broadcaster) *ActiveUserService {
	return &ActiveUserService{redis: redis, broadcaster: broadcaster, entropy: entropy.RealSource{}}
}

func (services *ActiveUserService) IsActiveUser(ctx context.Context, id string) bool {
	_, err := services.redis.PrimaryClient.ZScore(ctx, cache.Constants.ActiveUsersZSet, id).Result()
	return err == nil
}

func (services *ActiveUserService) GetActiveCount(ctx context.Context) (int64, error) {
	defer perf.WithContext(ctx).Log()

	expireBefore := services.entropy.GetTime().Add(-ActiveUserMaxAge)
	expireBeforeStr := fmt.Sprintf("%d", expireBefore.UnixMilli())

	removed, err := services.redis.PrimaryClient.ZRemRangeByScore(ctx, cache.Constants.ActiveUsersZSet, "-inf", expireBeforeStr).Result()
	if err != nil {
		return 0, serrors.New("get expired active users by range", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "expired users with keys", "count", removed, "expireBefore", expireBefore)
	}

	count, err := services.redis.PrimaryClient.ZCard(ctx, cache.Constants.ActiveUsersZSet).Result()
	if err != nil {
		return 0, serrors.New("count active users", err)
	}

	slog.InfoContext(ctx, "selected active users count", "count", count)
	return count, nil
}

func (services *ActiveUserService) RetainActiveUser(ctx context.Context, id string) error {
	defer perf.WithContext(ctx).Log()

	updtTime := float64(services.entropy.GetTime().UnixMilli())

	_, err := services.redis.PrimaryClient.ZAddXX(ctx, cache.Constants.ActiveUsersZSet, redis.Z{Score: updtTime, Member: id}).Result()
	if err != nil {
		return serrors.New("retain active user", err, "id", id)
	}

	slog.InfoContext(ctx, "retained active user", "id", id)
	return nil
}

func (services *ActiveUserService) AddActiveUser(ctx context.Context, id string) (int64, error) {
	defer perf.WithContext(ctx).Log()

	updtTime := float64(services.entropy.GetTime().UnixMilli())

	_, err := services.redis.PrimaryClient.ZAddNX(ctx, cache.Constants.ActiveUsersZSet, redis.Z{Score: updtTime, Member: id}).Result()
	if err != nil {
		return 0, serrors.New("add active user", err, "id", id)
	}
	slog.InfoContext(ctx, "added active user", "id", id)

	count, err := services.GetActiveCount(ctx)
	if err != nil {
		return 0, serrors.New("get active user count after adding user", err, "id", id)
	}

	services.broadcaster.BroadcastActiveCount(ctx, count)
	return count, nil
}

func (services *ActiveUserService) RemoveActiveUser(ctx context.Context, id string) (int64, error) {
	defer perf.WithContext(ctx).Log()

	res, err := services.redis.PrimaryClient.ZRem(ctx, cache.Constants.ActiveUsersZSet, id).Result()
	if err != nil {
		return 0, serrors.New("remove active user", err, "id", id)
	}
	if res > 0 {
		slog.InfoContext(ctx, "removed active user", "id", id)
	} else {
		slog.WarnContext(ctx, "did not remove active user", "id", id)
	}

	count, err := services.GetActiveCount(ctx)
	if err != nil {
		return 0, serrors.New("get active user count after removing user", err, "id", id)
	}

	services.broadcaster.BroadcastActiveCount(ctx, count)
	return count, nil
}
