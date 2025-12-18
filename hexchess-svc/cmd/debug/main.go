package main

import (
	"context"
	"flag"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"log/slog"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// scripts to easily view any protobuf serialized record in the database in text format for debugging

var mode = flag.String("mode", "move-sequence", "dump mode to execute")
var value = flag.String("value", "70", "the value to fetch")

func main() {
	util.InitLoggers(nil)
	util.InitEnv()

	dbURL := os.Getenv("DB_URL")

	ctx := context.WithValue(context.Background(), util.Trace, "seed-databases-script")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		util.LogFatalErr("create pool", err)
	}
	defer pool.Close()

	q := db.New(pool)

	switch *mode {
	case "move-sequence":
		id, err := strconv.Atoi(*value)
		if err != nil {
			util.LogFatalErr("parse id os arg", err)
		}

		pbMoveHist, err := svc.GetReplayMoveHistory(ctx, q, int64(id))
		if err != nil {
			util.LogFatalErr("get replay move list", err)
		}

		for _, pbStep := range pbMoveHist.Steps {
			game, err := chess.DeserializeGame(pbStep.Game)
			if err != nil {
				util.LogFatalErr("map game", err)
			}
			hm := chess.DeserializeHistMove(pbStep.Move)
			fmt.Printf("game with move: %s: %s\n", hm.String(), game.Board.String())
		}
	}
}
