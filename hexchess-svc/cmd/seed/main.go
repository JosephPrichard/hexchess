package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"log"
	"log/slog"
	"math/rand"
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

type ChallengeInst struct {
	ChallengerID int64     `json:"challengerId"`
	ChallengeeID int64     `json:"challengeeId"`
	Mode         string    `json:"mode"`
	StartColor   string    `json:"startColor"`
	MadeOn       time.Time `json:"madeOn"`
}

type GameResult struct {
	WhiteID            int64  `json:"whiteId"`
	BlackID            int64  `json:"blackId"`
	ReplayCause        string `json:"cause"`
	ReplayResult       string `json:"result"`
	ReplayMode         string `json:"mode"`
	SerializedMoveHist []byte
}

func main() {
	start := time.Now()

	challenges := readTestdataFile[ChallengeInst]("test/challenge_insts.json")
	gameResults := readTestdataFile[GameResult]("test/game_results.json")
	userInsts := readTestdataFile[svc.UserInst]("test/user_insts.json")

	logutil.InitLoggers(nil)
	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakePostgres(pool)

	slog.InfoContext(ctx, "connecting to rdb db", "rdbPrimaryURL", rdbPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL}, db.DefaultRedisNames)

	state := &svc.State{Redis: rdb, Postgres: pdb}
	defer state.Close()

	_, err = pool.Exec(ctx, `
		TRUNCATE TABLE users, replays, challenges
    	RESTART IDENTITY
		CASCADE;`)
	if err != nil {
		logutil.FatalErr("drop schema", err)
	}

	if err := rdb.Cache.FlushAll(ctx).Err(); err != nil {
		logutil.FatalErr("flush rdb", err)
	}

	if _, err := state.BatchInsertUsers(ctx, userInsts); err != nil {
		logutil.FatalErr("insert users", err)
	}
	for _, chInst := range challenges {
		if err := state.InsertChallenge(ctx, mapChallengeInst(chInst)); err != nil {
			logutil.FatalErr("insert challenge", err)
		}
	}
	if err := insertRandomizedGameResult(ctx, state, gameResults); err != nil {
		logutil.FatalErr("insert game results", err)
	}
	if err := state.SyncLeaderboard(ctx); err != nil {
		logutil.FatalErr("sync leaderboard", err)
	}

	log.Printf("finished seeding databases: %v", time.Since(start))
}

func mapChallengeInst(chInst ChallengeInst) svc.ChallengeInst {
	return svc.ChallengeInst{
		ChallengerID: chInst.ChallengerID,
		ChallengeeID: chInst.ChallengeeID,
		Mode:         svc.ExpectGameMode(chInst.Mode),
		StartColor:   svc.ExpectColor(chInst.StartColor),
		MadeOn:       chInst.MadeOn,
	}
}

func insertRandomizedGameResult(ctx context.Context, state *svc.State, gameResults []GameResult) error {
	timeAt := time.Now().Add(-1 * time.Hour * 24 * 100)

	a := gameResults
	rand.Shuffle(len(a), func(i, j int) {
		a[i], a[j] = a[j], a[i]
	})

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

		if _, err = state.InsertGameResultTx(ctx, timeAt.Add(time.Duration(i)*time.Hour*24), svc.GameResult{
			WhiteID:            params.WhiteID,
			BlackID:            params.BlackID,
			ReplayCause:        svc.ExpectReplayCause(params.ReplayCause),
			ReplayResult:       svc.ExpectReplayResult(params.ReplayResult),
			ReplayMode:         svc.ExpectGameMode(params.ReplayMode),
			SerializedMoveHist: params.SerializedMoveHist,
		}); err != nil {
			return fmt.Errorf("insert game result: %w", err)
		}
	}
	return nil
}
