package main

import (
	"context"
	"flag"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"log/slog"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// scripts to easily view any protobuf serialized record in the database in text format for debugging

var mode = flag.String("mode", "move-sequence", "dump mode to execute")
var value = flag.String("value", "70", "the value to fetch")

func main() {
	logutil.InitLoggers(nil)
	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	postgres := db.MakePostgres(pool)
	defer postgres.Close()

	switch *mode {
	case "move-sequence":
		id, err := strconv.Atoi(*value)
		if err != nil {
			logutil.FatalErr("parse ID os arg", err)
		}

		s := svc.State{Postgres: postgres}
		pbMoveHist, err := s.GetReplayMoveHistory(ctx, int64(id))
		if err != nil {
			logutil.FatalErr("get replay move list", err)
		}

		for _, pbStep := range pbMoveHist.Steps {
			game, err := chess.DeserializeGame(pbStep.Game)
			if err != nil {
				logutil.FatalErr("map game", err)
			}
			hm := chess.DeserializeHistMove(pbStep.Move)
			fmt.Printf("game with move: %s: %s\n", hm.String(), game.Board.String())
		}
	}
}
