package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/chess"
	"hexchess-svc/cmd"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/logs"
	"hexchess-svc/static"
	"log"
	"log/slog"
	"math/rand"
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
	var moveList []chess.PieceMove

	// generates a random move list for mock data between length 35 and 45
	for range rand.Intn(10) + 35 {
		game.InitPieceMoves()

		var fpm chess.PieceMoves
		for _, pm := range game.GetCurrMoves() {
			if len(pm.Moves) > 0 {
				fpm = pm
				break
			}
		}
		if len(fpm.Moves) == 0 {
			return fmt.Errorf("expected at least one move, got none for game: %v", game)
		}
		pm := chess.PieceMove{From: fpm.From, To: fpm.Moves[0]}

		game.MakeMove(pm.From, pm.To)
		moveList = append(moveList, pm)
	}

	b, err := json.Marshal(moveList)
	if err != nil {
		return err
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

	cmd.InitEnv()

	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	ctx := context.WithValue(context.Background(), logs.TraceKey, "seed-stores-script")

	slog.InfoContext(ctx, "connecting to postgres db", "user", dbUser, "name", dbName, "port", dbPort)
	pool, err := pgxpool.New(ctx, fmt.Sprintf("user=%s dbname=%s password=%s port=%s", dbUser, dbName, dbPass, dbPort))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)

	slog.InfoContext(ctx, "connecting to redis db", "host", redisHost, "port", redisPort)
	rdb := data.MakeRdb(redisHost + ":" + redisPort)
	defer rdb.Close()

	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		log.Fatalf("failed to drop schema: %v", err)
	}
	if _, err := pool.Exec(context.Background(), db.CreateSchema); err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}

	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("FLUSHALL"); err != nil {
		log.Fatalf("failed to flush redis: %v", err)
	}

	users, err := data.BatchInsertUsers(ctx, q, userInsts)
	if err != nil {
		log.Fatalf("failed to insert users: %v", err)
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
		log.Fatalf("failed to insert challenges and replys: %v", err)
	}

	log.Printf("finished seeding databases: %v", time.Now().Sub(start))
}
