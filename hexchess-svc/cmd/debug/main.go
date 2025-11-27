package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log/slog"
	"os"
	"strconv"
)

// scripts to easily view any protobuf serialized record in the database in text format for debugging

var mode = flag.String("mode", "move-sequence", "Dump mode to execute.")
var value = flag.String("value", "70", "The value to fetch.")

func main() {
	util.InitLoggers(nil)
	util.InitEnv()

	dbURL := os.Getenv("DB_URL")

	ctx := context.WithValue(context.Background(), util.Trace, "seed-stores-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		util.LogFatalErr("failed to create pool", err)
	}
	defer pool.Close()

	q := db.New(pool)

	switch *mode {
	case "move-sequence":
		id, err := strconv.Atoi(*value)
		if err != nil {
			util.LogFatalErr("failed to parse id os arg", err)
		}

		pbMoveHist, err := data.GetReplayMoveHistory(ctx, q, int64(id))
		if err != nil {
			util.LogFatalErr("failed to get replay move list", err)
		}

		for _, pbStep := range pbMoveHist.Steps {
			game, err := chess.DeserializeGame(pbStep.Game)
			if err != nil {
				util.LogFatalErr("failed to map game", err)
			}
			hm := chess.DeserializeHistMove(pbStep.Move)
			fmt.Printf("game with move: %s: %s\n", hm.String(), game.Board.String())
		}
	}
}
