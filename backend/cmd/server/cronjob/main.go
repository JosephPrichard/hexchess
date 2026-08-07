package main

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/service/file"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/utils/config"
	"log/slog"
	"os"
	"time"

	"hexchess-svc/database"
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

	databaseClient := database.NewDatabase(ctx, database.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL,
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer databaseClient.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	aws := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		ActiveProfile: cfg.Profile,
		Names:         cloud.AWSNames{S3ProfileBucket: cfg.ProfileBucket},
		AWSRegion:     cfg.AwsRegion,
		AWSEndpoint:   cfg.AwsEndpoint,
		AWSUsername:   cfg.AwsUsername,
		AWSPassword:   cfg.AwsPassword,
	})

	leaderboardSvc := leaderboard.NewLeaderboardService(redisClient, databaseClient.Querier())
	profileSvc := file.NewOrphanService(aws, databaseClient.Querier())

	var err error

	switch job {
	case SyncLeaderboardJobName:
		err = leaderboardSvc.SyncLeaderboard(ctx)
	case ClearS3OrphansJobName:
		err = profileSvc.ClearOrphanFiles(ctx, file.PageLength)
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
