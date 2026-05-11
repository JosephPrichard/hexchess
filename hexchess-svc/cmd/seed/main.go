package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/hexchess"
	"hexchess-svc/model"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"hexchess-svc/assets"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	svc "hexchess-svc/service"
	"hexchess-svc/util/logutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

func readTestdataFile[V any](filename string) []V {
	b, err := assets.TestData.ReadFile(filename)
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

	shutdown := logutil.InitLoggers(logutil.LogConfig{})
	defer shutdown(context.Background())

	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbCacheURL := os.Getenv("REDIS_CACHE_URL")

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakeDB(pool)

	addrs := db.RedisAddrs{CacheAddr: rdbCacheURL}
	slog.InfoContext(ctx, "connecting to redis db", "addrs", addrs)
	rdb := db.MakeRdb(addrs, nil)

	services := svc.MakeHexchessServices(svc.Setup{
		DB:      pdb,
		Querier: pdb.Querier(),
		Redis:   rdb,
	})
	defer services.Close()

	_, err = pool.Exec(ctx, `
		TRUNCATE TABLE users, replays, challenges, outbox_queue, tournaments, tournament_matches, tournament_participants, user_mode_elos, replay_move_histories
    	RESTART IDENTITY
		CASCADE;`)
	if err != nil {
		logutil.FatalErr("drop schema", err)
	}

	if err := rdb.Cache.FlushAll(ctx).Err(); err != nil {
		logutil.FatalErr("flush rdb", err)
	}

	// we need to inserts users before we can insert challenges and game results (they reference users)
	if _, err := services.BatchInsertUsers(ctx, userInsts); err != nil {
		logutil.FatalErr("insert users", err)
	}
	if err := insertChallenges(ctx, services, challenges); err != nil {
		logutil.FatalErr("insert challenges", err)
	}
	// performs stat updates to users, must happen before we sync the leaderboard (which copies from users table into redis)
	if err := insertRandomizedGameResults(ctx, services, gameResults); err != nil {
		logutil.FatalErr("insert game results", err)
	}
	if err := services.SyncLeaderboard(ctx); err != nil {
		logutil.FatalErr("jobs leaderboard", err)
	}

	log.Printf("finished seeding databases: %v", time.Since(start))
}

func insertChallenges(ctx context.Context, services svc.HexchessAPI, insts []ChallengeInst) error {
	for _, chInst := range insts {
		if _, err := services.InsertChallenge(ctx, svc.ChallengeInst{
			ChallengerID: chInst.ChallengerID,
			ChallengeeID: chInst.ChallengeeID,
			Mode:         model.ExpectGameMode(chInst.Mode),
			StartColor:   model.ExpectColor(chInst.StartColor),
			MadeOn:       chInst.MadeOn,
		}); err != nil {
			return err
		}
	}
	return nil
}

func insertRandomizedGameResults(ctx context.Context, services svc.HexchessAPI, gameResults []GameResult) error {
	timeAt := time.Now().Add(-1 * time.Hour * 24 * 100)

	a := gameResults
	rand.Shuffle(len(a), func(i, j int) {
		a[i], a[j] = a[j], a[i]
	})

	eg, egCtx := errgroup.WithContext(ctx)

	for gameIdx, params := range gameResults {
		eg.Go(func() error {
			mode := model.ExpectGameMode(params.ReplayMode)

			moveSeq, err := svc.RandomMoveHistSeq(mode, hexchess.MakeStartGame(), 10, 30)
			if err != nil {
				return fmt.Errorf("generate random move seq: %w", err)
			}
			moveHistBlob, err := hexchess.MarshalMoveHistory(hexchess.InitialBoard(), moveSeq)
			if err != nil {
				return fmt.Errorf("marshal move history to s3: %w", err)
			}

			changeSet, err := services.InsertGameResultTx(egCtx, svc.GameResult{
				GameID:       svc.MakeGameID(),
				WhiteID:      params.WhiteID,
				BlackID:      params.BlackID,
				ReplayCause:  model.ExpectReplayCause(params.ReplayCause),
				ReplayResult: model.ExpectReplayResult(params.ReplayResult),
				ReplayMode:   mode,
				InsertedTime: timeAt.Add(time.Duration(gameIdx) * time.Hour * 24),
				TurnCount:    len(moveSeq),
			})
			if err != nil {
				return fmt.Errorf("insert game result: %w", err)
			}
			// note: don't forget to insert the move history - it exists outside of the game result tx
			if err = services.UpsertReplayMoveHistories(ctx, changeSet.ReplayID, moveHistBlob); err != nil {
				return fmt.Errorf("insert replay move histories: %w", err)
			}
			return nil
		})
	}

	return eg.Wait()
}
