package main

import (
	"context"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	start := time.Now()

	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logutil.LogFatalErr("open log file", err)
	}
	defer f.Close()

	logutil.InitLoggers(f)
	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	redisPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), logutil.Trace, "sync-leaderboard-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.LogFatalErr("create pool", err)
	}
	defer pool.Close()
	q := db.New(pool)

	slog.InfoContext(ctx, "connecting to redis db", "redisPrimaryURL", redisPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: redisPrimaryURL}, db.DefaultRedisNames)
	defer rdb.Close()

	dbs := &db.Databases{Pdb: db.MakePostgres(q, pool), Rdb: rdb}
	if err := svc.SyncLeaderboard(ctx, dbs); err != nil {
		logutil.LogFatalErr("sync leaderboard", err)
	}
	log.Printf("finished syncing leaderboard: %v", time.Now().Sub(start))
}
