package sync

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/app/data"
	"hexchess-svc/app/util"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"log"
	"log/slog"
	"os"
)

func main() {
	cmd.InitEnv()

	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	ctx := context.WithValue(context.Background(), util.TraceKey, "sync-leaderboard-script")

	slog.Info("connecting to postgres db", "user", dbUser, "name", dbName, "port", dbPort)
	pool, err := pgxpool.New(ctx, fmt.Sprintf("user=%s dbname=%s password=%s port=%s", dbUser, dbName, dbPass, dbPort))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)

	redisAddr := redisHost + ":" + redisPort

	slog.Info("connecting to redis db", "host", redisHost, "port", redisPort)
	rdb := data.MakeRdbPool(redisAddr)

	afterID := int64(0)
	for {
		rows, err := q.SelectEloListAfterID(ctx, db.SelectEloListAfterIDParams{AfterID: afterID, Limit: 20})
		if err != nil {
			log.Fatalf("failed to select elo list: %v", err)
		}
		if len(rows) == 0 {
			break
		}
		var csList []data.UpdtLbChangeSet
		for i, row := range rows {
			if i == len(rows)-1 {
				afterID = row.ID
			}
			csList = append(csList, data.UpdtLbChangeSet{ID: row.ID, EloDiff: row.Elo})
		}
		if err := data.SetLeaderboard(ctx, rdb, csList...); err != nil {
			log.Fatalf("failed to incr leaderboard: %v", err)
		}
	}
}
