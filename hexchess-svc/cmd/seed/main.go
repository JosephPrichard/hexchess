package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/static"
	"hexchess-svc/util"
	"log"
	"log/slog"
	"os"
	"time"
)

func readMockFile[V any](filename string) []V {
	b, err := static.Mocks.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	var v []V
	if err := json.Unmarshal(b, &v); err != nil {
		log.Fatal(err)
	}
	return v
}

func insertReplay(ctx context.Context, q *db.Queries, r data.ReplayInst) error {
	game := chess.MakeStartGame()
	moveList, err := chess.RandomMoveList(game, 35, 45)
	if err != nil {
		return fmt.Errorf("failed to generate random move list: %w", err)
	}

	b, err := json.Marshal(moveList)
	if err != nil {
		return fmt.Errorf("failed to marshal move list: %w", err)
	}
	r.MoveListJSON = string(b)

	_, err = data.InsertReplay(ctx, q, r)
	return err
}

func main() {
	start := time.Now()

	challenges := readMockFile[data.ChallengeInst]("mocks/challenges.json")
	replays := readMockFile[data.ReplayInst]("mocks/replays.json")
	userInsts := readMockFile[data.UserInst]("mocks/users.json")

	util.InitLoggers(nil)
	util.InitEnv()

	dbURL := os.Getenv("DB_URL")
	redisPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), util.Trace, "seed-stores-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		util.LogFatal("failed to create pool", "err", err)
	}
	defer pool.Close()

	q := db.New(pool)

	slog.InfoContext(ctx, "connecting to redis db", "redisPrimaryURL", redisPrimaryURL)
	rdb := data.MakeRdb(redisPrimaryURL, "")
	defer rdb.Close()

	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		util.LogFatal("failed to drop schema", "err", err)
	}
	if _, err := pool.Exec(context.Background(), db.CreateSchema); err != nil {
		util.LogFatal("failed to create schema", "err", err)
	}

	conn := rdb.Primary.Get()
	defer conn.Close()

	if _, err := conn.Do("FLUSHALL"); err != nil {
		util.LogFatal("failed to flush redis", "err", err)
	}

	users, err := data.BatchInsertUsers(ctx, q, userInsts)
	if err != nil {
		util.LogFatal("failed to insert users", "err", err)
	}
	var changes []data.UpdtLbChangeSet
	for _, u := range users {
		changes = append(changes, data.UpdtLbChangeSet{ID: u.ID, EloDiff: u.Elo})
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return data.SetLeaderboard(egCtx, rdb, changes...)
	})
	for _, c := range challenges {
		eg.Go(func() error {
			return data.InsertChallenge(egCtx, q, c)
		})
	}
	for _, r := range replays {
		eg.Go(func() error {
			return insertReplay(egCtx, q, r)
		})
	}

	if err := eg.Wait(); err != nil {
		util.LogFatal("failed to insert challenges and replys", "err", err)
	}

	log.Printf("finished seeding databases: %v", time.Now().Sub(start))
}
