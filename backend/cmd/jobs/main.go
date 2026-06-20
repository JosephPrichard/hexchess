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
	defer shutdown()

	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")

	pdb := db.MakeDB(db.MakePgPool(ctx, dbURL))
	defer pdb.Close()

	rdb := db.MakeRedis(db.RedisAddrs{SorAddr: rdbSorNodes}, nil)
	defer rdb.Close()

	services := svc.MakeHexchessServices(svc.SetupService{DB: pdb, Redis: rdb})

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
