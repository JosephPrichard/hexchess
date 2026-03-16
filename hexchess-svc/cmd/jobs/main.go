package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"time"

	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"

	"github.com/jackc/pgx/v5/pgxpool"
)

var jobName = flag.String("job", "jobs", "dump mode to execute")

func main() {
	start := time.Now()

	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logutil.FatalErr("open log file", err)
	}
	defer f.Close()

	logutil.InitLoggers(f)
	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), logutil.Trace, "jobs-runner")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakeDB(pool)

	slog.InfoContext(ctx, "connecting to rdb db", "rdbPrimaryURL", rdbPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL}, db.DefaultRedisNames)

	services := &svc.Services{
		DB:      pdb,
		Queries: pdb.Queries(),
		Redis:   rdb,
	}
	defer services.Close()

	switch *jobName {
	case "sync-leaderboard":
		if err := services.SyncLeaderboard(ctx); err != nil {
			logutil.FatalErr("failed to execute sync leaderboard job", err)
		}
		log.Printf("finished syncing leaderboard job: %v", time.Since(start))
	case "clear-s3-orphans":
		services.ClearBucketOrphans(ctx, svc.PageLength)
		log.Printf("finished clear bucket orphans job: %v", time.Since(start))
	default:
		log.Fatalf("unknown job: %s", *jobName)
	}
}
