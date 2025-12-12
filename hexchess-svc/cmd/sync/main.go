package main

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/dpl"
	"hexchess-svc/infra"
	"hexchess-svc/util"
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
		util.LogFatalErr("open log file", err)
	}
	defer f.Close()

	util.InitLoggers(f)
	util.InitEnv()

	dbURL := os.Getenv("DB_URL")
	redisPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), util.Trace, "sync-leaderboard-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		util.LogFatalErr("create pool", err)
	}
	defer pool.Close()
	q := db.New(pool)

	slog.InfoContext(ctx, "connecting to redis db", "redisPrimaryURL", redisPrimaryURL)
	rdb := infra.MakeRdb(redisPrimaryURL, "")
	defer rdb.Close()

	databases := &infra.Databases{Pdb: infra.MakePostgres(q, pool), Rdb: rdb}

	if err := dpl.SyncLeaderboard(ctx, databases); err != nil {
		util.LogFatalErr("sync leaderboard", err)
	}
	log.Printf("finished syncing leaderboard: %v", time.Now().Sub(start))
}
