package main

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/utils/config"
	"log/slog"
	"os"
	"time"

	"hexchess-svc/db"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/logutil"
)

const (
	SyncLeaderboardJobName = "sync-leaderboard"
	ClearS3OrphansJobName  = "clear-s3-orphans"

	ServiceName = "hexchess-job-runner"
)

func main() {
	slog.Info("begin cronjob app")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	start := time.Now()

	cfg := config.Load()
	job := os.Getenv("JOB_NAME")

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	database := db.NewDatabase(ctx, db.DatabaseConfig{
		Dsn:           cfg.DbURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer database.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:     cfg.RedisPrimaryNodes,
		PrimaryUsername: cfg.RedisPrimaryUsername,
		PrimaryPassword: cfg.RedisPrimaryPassword,
		ActiveProfile:   cfg.Profile,
	})
	defer redisClient.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		Database: database,
		Redis:    redisClient,
	})

	var err error

	switch job {
	case SyncLeaderboardJobName:
		err = services.SyncLeaderboard(ctx)
	case ClearS3OrphansJobName:
		err = services.ClearOrphanFiles(ctx, svc.PageLength)
	default:
		logutil.Fatal("unknown job", nil, "job", job)
	}

	timeTaken := time.Since(start)

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		slog.Info("job execution timed out", "job", job, "timeTaken", timeTaken)
	case err != nil:
		logutil.Fatal("execute job", err, "job", job, "timeTaken", timeTaken)
	default:
		slog.Info("successfully executed job", "job", job, "timeTaken", timeTaken)
	}
}
