package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/ext"
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
	WhiteID      int64  `json:"whiteId"`
	BlackID      int64  `json:"blackId"`
	ReplayCause  string `json:"cause"`
	ReplayResult string `json:"result"`
	ReplayMode   string `json:"mode"`
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
	awsSecretID := os.Getenv("AWS_SECRET_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_KEY")
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakePostgres(pool)

	slog.InfoContext(ctx, "connecting to rdb db", "rdbPrimaryURL", rdbPrimaryURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL}, db.DefaultRedisNames)

	aws, err := ext.MakeAwsClients(context.Background(), ext.AwsConfig{
		AwsDefaultRegion: awsDefaultRegion,
		AwsSecretKey:     awsSecretID,
		AwsSecretID:      awsSecretKey,
		AwsEndpoint:      awsEndpoint,
	})
	if err != nil {
		logutil.FatalErr("make aws clients", err)
	}

	state := &svc.State{Redis: rdb, Postgres: pdb, Aws: aws}
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
		logutil.FatalErr("jobs leaderboard", err)
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
		cs, err := state.InsertGameResultTx(ctx, timeAt.Add(time.Duration(i)*time.Hour*24), svc.GameResult{
			WhiteID:      params.WhiteID,
			BlackID:      params.BlackID,
			ReplayCause:  svc.ExpectReplayCause(params.ReplayCause),
			ReplayResult: svc.ExpectReplayResult(params.ReplayResult),
			ReplayMode:   svc.ExpectGameMode(params.ReplayMode),
		})
		if err != nil {
			return fmt.Errorf("insert game result: %w", err)
		}

		game := chess.MakeStartGame()
		moveSeq, err := chess.RandomMoveSeq(game, 10, 30)
		if err != nil {
			return fmt.Errorf("generate random move seq: %w", err)
		}
		if err := state.PutReplayMoveSeq(ctx, cs.ReplayID, game.Board, moveSeq); err != nil {
			return err
		}
	}
	return nil
}
