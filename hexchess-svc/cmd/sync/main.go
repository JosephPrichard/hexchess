package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/cmd"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log"
	"log/slog"
	"os"
	"time"
)

func main() {
	start := time.Now()

	util.InitLoggers(nil)
	cmd.InitEnv()

	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	ctx := context.WithValue(context.Background(), util.Trace, "sync-leaderboard-script")

	slog.InfoContext(ctx, "connecting to postgres db", "user", dbUser, "name", dbName, "port", dbPort)
	pool, err := pgxpool.New(ctx, fmt.Sprintf("user=%s dbname=%s password=%s port=%s", dbUser, dbName, dbPass, dbPort))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()
	q := db.New(pool)

	redisAddr := redisHost + ":" + redisPort
	slog.InfoContext(ctx, "connecting to redis db", "host", redisHost, "port", redisPort)
	rdb := data.MakeRdb(redisAddr, "")
	defer rdb.Close()

	stores := data.Stores{PgDB: data.MakeDbClient(q, pool), Rdb: rdb}

	if err := data.SyncLeaderboard(ctx, stores); err != nil {
		log.Fatalf("failed to sync leaderboard: %v", err)
	}
	log.Printf("finished syncing leaderboard: %v", time.Now().Sub(start))
}
