package main

import (
	"context"
	"flag"
	"hexchess-svc/utils/config"
	"log/slog"
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

var jobName = flag.String("job", SyncLeaderboardJobName, "job to execute")

func main() {
	ctx := context.Background()

	cfg := config.Load()

	start := time.Now()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	primaryDB := db.NewPostgresDB(ctx, db.PrimaryQuerierFactory, db.PoolConfig{
		Dsn:           cfg.PrimaryDbURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer primaryDB.Close()

	primaryRedis := db.NewRedis(ctx, db.RedisConfig{
		PrimaryAddr:     cfg.RedisPrimaryNodes,
		PrimaryUsername: cfg.RedisPrimaryUsername,
		PrimaryPassword: cfg.RedisPrimaryPassword,
		ActiveProfile:   cfg.Profile,
	})
	defer primaryRedis.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		PrimaryDB: primaryDB,
		Redis:     primaryRedis,
	})

	switch *jobName {
	case SyncLeaderboardJobName:
		if err := services.SyncLeaderboard(ctx); err != nil {
			logutil.Fatal("failed to execute sync leaderboard job", err)
		}
		slog.InfoContext(ctx, "finished syncing leaderboard job", "timeTaken", time.Since(start).String())
	case ClearS3OrphansJobName:
		services.ClearOrphanFiles(ctx, svc.PageLength)
		slog.InfoContext(ctx, "finished clear s3 orphans job", "timeTaken", time.Since(start).String())
	default:
		logutil.Fatal("unknown job", nil, "job", *jobName)
	}
}
