package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SyncLeaderboardJobName = "sync-leaderboard"
	ClearS3OrphansJobName  = "clear-s3-orphans"
)

var jobName = flag.String("job", "", "job to execute")

func main() {
	ctx := context.WithValue(context.Background(), logutil.Trace, "jobs-runner")

	start := time.Now()

	shutdown := logutil.InitLoggers(logutil.LogConfig{})
	defer shutdown(ctx)

	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")

	services := makeServices(ctx, dbURL, rdbSorNodes)

	switch *jobName {
	case SyncLeaderboardJobName:
		if err := services.SyncLeaderboard(ctx); err != nil {
			logutil.FatalErr("failed to execute sync leaderboard job", err)
		}
		slog.InfoContext(ctx, "finished syncing leaderboard job", "timeTaken", time.Since(start))
	case ClearS3OrphansJobName:
		services.ClearOrphanFiles(ctx, svc.PageLength)
		slog.InfoContext(ctx, "finished clear s3 orphans job", "timeTaken", time.Since(start))
	default:
		log.Fatalf("unknown job: %s", *jobName)
	}
}

func makeServices(ctx context.Context, dbURL string, rdbSorNodes []string) *svc.HexchessServices {
	var setup svc.SetupService

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	setup.DB = db.MakeDB(pool)

	addrs := db.RedisAddrs{SorAddr: rdbSorNodes}
	slog.InfoContext(ctx, "connecting to redis db", "addrs", addrs)
	setup.Redis = db.MakeRedis(addrs, nil)

	return svc.MakeHexchessServices(setup)
}
