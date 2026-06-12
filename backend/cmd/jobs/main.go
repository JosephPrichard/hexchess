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
	"hexchess-svc/lib/logutil"
	"hexchess-svc/service"

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

	shutdown := logutil.InitLoggers(logutil.LogConfig{})
	defer shutdown(context.Background())

	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbCacheURL := os.Getenv("REDIS_CACHE_NODES")

	ctx := context.WithValue(context.Background(), logutil.Trace, "jobs-runner")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakeDB(pool)

	addrs := db.RedisAddrs{CacheAddr: rdbCacheURL}
	slog.InfoContext(ctx, "connecting to redis db", "addrs", addrs)
	rdb := db.MakeRedis(addrs, nil)

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:    pdb,
		Redis: rdb,
	})
	defer testinfra.Close()

	switch *jobName {
	case "sync-leaderboard":
		if err := services.SyncLeaderboard(ctx); err != nil {
			logutil.FatalErr("failed to execute sync leaderboard job", err)
		}
		log.Printf("finished syncing leaderboard job: %v", time.Since(start))
	case "clear-s3-orphans":
		services.ClearOrphanFiles(ctx, svc.PageLength)
		log.Printf("finished clear bucket orphans job: %v", time.Since(start))
	default:
		log.Fatalf("unknown job: %s", *jobName)
	}
}
