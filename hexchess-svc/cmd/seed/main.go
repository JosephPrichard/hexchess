package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func readTestdataFile[V any](filename string) []V {
	b, err := assets.Mocks.ReadFile(filename)
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

	challenges := readTestdataFile[svc.ChallengeInst]("test/challenge_insts.json")
	gameResults := readTestdataFile[svc.GameResult]("test/game_results.json")
	userInsts := readTestdataFile[svc.UserInst]("test/user_insts.json")

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
	pdb := db.MakePostgres(q, pool)

	slog.InfoContext(ctx, "connecting to redis db", "redisPrimaryURL", redisPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: redisPrimaryURL}, db.DefaultRedisNames)
	defer rdb.Close()

	dbs := &db.Databases{Rdb: rdb, Pdb: pdb}

	_, err = pool.Exec(ctx, `
		TRUNCATE TABLE users, replays, challenges
    	RESTART IDENTITY
		CASCADE;`)
	if err != nil {
		util.LogFatalErr("drop schema", err)
	}

	if err := rdb.Cache.FlushAll(ctx).Err(); err != nil {
		util.LogFatalErr("flush redis", err)
	}

	if _, err := svc.BatchInsertUsers(ctx, q, userInsts); err != nil {
		util.LogFatalErr("insert users", err)
	}
	for _, chInst := range challenges {
		if err := svc.InsertChallenge(ctx, q, chInst); err != nil {
			util.LogFatalErr("insert challenge", err)
		}
	}
	if err := insertRandomizedGameResult(ctx, pdb, gameResults); err != nil {
		util.LogFatalErr("insert game results", err)
	}
	if err := svc.SyncLeaderboard(ctx, dbs); err != nil {
		util.LogFatalErr("sync leaderboard", err)
	}

	log.Printf("finished seeding databases: %v", time.Now().Sub(start))
}

func insertRandomizedGameResult(ctx context.Context, pdb *db.PostgreSQL, gameResults []svc.GameResult) error {
	timeAt := time.Now().Add(-1 * time.Hour * 24 * 100)
	util.Shuffle(gameResults)

	for i, params := range gameResults {
		game := chess.MakeStartGame()

		moveSeq, err := chess.RandomMoveSeq(game, 10, 30)
		if err != nil {
			return fmt.Errorf("generate random move list: %w", err)
		}

		moveHistBytes, err := chess.MarshalMoveHistory(game.Board, moveSeq)
		if err != nil {
			return fmt.Errorf("marshal move history: %w", err)
		}
		params.SerializedMoveHist = moveHistBytes

		if _, err = svc.InsertGameResultTx(ctx, pdb, timeAt.Add(time.Duration(i)*time.Hour*24), params); err != nil {
			return fmt.Errorf("insert game result: %w", err)
		}
	}
	return nil
}
