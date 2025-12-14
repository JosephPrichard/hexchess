package main

import (
	"context"
	"encoding/json"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/infra"
	"hexchess-svc/static"
	"hexchess-svc/svc"
	"hexchess-svc/util"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
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

func main() {
	start := time.Now()

	challenges := readMockFile[svc.ChallengeInst]("test/challenge_insts.json")
	gameResults := readMockFile[svc.GameResult]("test/game_results.json")
	userInsts := readMockFile[svc.UserInst]("test/user_insts.json")

	util.InitLoggers(nil)
	util.InitEnv()

	dbURL := os.Getenv("DB_URL")
	redisPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), util.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		util.LogFatalErr("create pool", err)
	}
	defer pool.Close()

	q := db.New(pool)
	pdb := infra.MakePostgres(q, pool)

	slog.InfoContext(ctx, "connecting to redis db", "redisPrimaryURL", redisPrimaryURL)
	rdb := infra.MakeRdb(infra.RedisAddrs{CacheAddr: redisPrimaryURL}, infra.DefaultRedisNames)
	defer rdb.Close()

	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		util.LogFatalErr("drop schema", err)
	}
	if _, err := pool.Exec(context.Background(), db.CreateSchema); err != nil {
		util.LogFatalErr("create schema", err)
	}

	if err := rdb.Cache.FlushAll(ctx).Err(); err != nil {
		util.LogFatalErr("flush redis", err)
	}

	users, err := svc.BatchInsertUsers(ctx, q, userInsts)
	if err != nil {
		util.LogFatalErr("insert users", err)
	}
	var changes []svc.UpdtLbChangeSet
	for _, u := range users {
		changes = append(changes, svc.UpdtLbChangeSet{ID: u.ID, EloDiff: u.Elo})
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return svc.SetLeaderboard(egCtx, rdb, changes...)
	})
	for _, c := range challenges {
		eg.Go(func() error {
			return svc.InsertChallenge(egCtx, q, c)
		})
	}

	timeAt := time.Now().Add(-1 * time.Hour * 24 * 100)
	var timesAt []time.Time
	for i := range gameResults {
		timesAt = append(timesAt, timeAt.Add(time.Duration(i)*time.Hour*24))
	}

	for i, params := range gameResults {
		eg.Go(func() error {
			game := chess.MakeStartGame()
			moveSeq, err := chess.RandomMoveSeq(game, 10, 30)
			if err != nil {
				util.LogFatalErr("generate random move list", err)
			}
			moveHistBytes, err := chess.MarshalMoveHistory(game.Board, moveSeq)
			if err != nil {
				util.LogFatalErr("marshal move history: %w", err)
			}
			params.SerializedMoveHist = moveHistBytes

			if _, err = svc.InsertGameResultTx(ctx, pdb, timesAt[i], params); err != nil {
				util.LogFatalErr("insert game result", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		util.LogFatalErr("insert challenges and replays", err)
	}

	log.Printf("finished seeding databases: %v", time.Now().Sub(start))
}
