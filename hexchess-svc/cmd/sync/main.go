package main

import (
	"context"
	"flag"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var jobName = flag.String("job", "sync", "dump mode to execute")

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

	ctx := context.WithValue(context.Background(), logutil.Trace, "sync-leaderboard-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	defer pool.Close()

	slog.InfoContext(ctx, "connecting to rdb db", "rdbPrimaryURL", rdbPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL}, db.DefaultRedisNames)
	defer rdb.Close()

	state := &svc.State{Postgres: db.MakePostgres(pool), Redis: rdb}

	switch *jobName {
	case "sync":
		if err := state.SyncLeaderboard(ctx); err != nil {
			logutil.FatalErr("sync leaderboard", err)
		}
		log.Printf("finished syncing leaderboard: %v", time.Now().Sub(start))
	default:
		log.Fatalf("unknown job: %s", *jobName)
	}
}
