package main

import (
	"context"
	"flag"
	"hexchess-svc/lib/dotenv"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"hexchess-svc/db"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/service"
)

const (
	SyncLeaderboardJobName = "sync-leaderboard"
	ClearS3OrphansJobName  = "clear-s3-orphans"

	ServiceName = "hexchess-job-runner"
)

var jobName = flag.String("job", "", "job to execute")

func main() {
	ctx := context.WithValue(context.Background(), logutil.Trace, "jobs-runner")

	dotenv.Load()

	dbURL := os.Getenv("DB_URL")
	profile := os.Getenv("PROFILE")
	awsRegion := os.Getenv("AWS_REGION")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	start := time.Now()

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	pool, closer := db.NewPgPool(ctx, db.PgConnectCfg{
		Dsn:           dbURL,
		ActiveProfile: profile,
		Region:        awsRegion,
	})
	defer closer()
	pdb := db.NewDB(pool)

	rdb, closer := db.NewRedis(ctx, db.RedisCfg{
		Addrs:         db.RedisAddrs{SorAddr: rdbSorNodes},
		ActiveProfile: profile,
	})
	defer closer()

	services := svc.NewHexchessServices(svc.SetupService{DB: pdb, Redis: rdb})

	switch *jobName {
	case SyncLeaderboardJobName:
		if err := services.SyncLeaderboard(ctx); err != nil {
			logutil.Fatal("failed to execute sync leaderboard job", err)
		}
		slog.InfoContext(ctx, "finished syncing leaderboard job", "timeTaken", time.Since(start))
	case ClearS3OrphansJobName:
		services.ClearOrphanFiles(ctx, svc.PageLength)
		slog.InfoContext(ctx, "finished clear s3 orphans job", "timeTaken", time.Since(start))
	default:
		log.Fatalf("unknown job: %s", *jobName)
	}
}
