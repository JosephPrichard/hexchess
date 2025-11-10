package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/util"
	"log/slog"
	"os"
	"strconv"
)

// scripts to easily view any protobuf serialized record in the database in text format for debugging

var mode = flag.String("mode", "move-list", "Dump mode to execute.")
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
	case "move-list":
		id, err := strconv.Atoi(*value)
		if err != nil {
			util.LogFatalErr("failed to parse id os arg", err)
		}

		b, err := data.GetReplayMoveHistory(ctx, q, int64(id))
		if err != nil {
			util.LogFatalErr("failed to get replay move list", err)
		}

		var pbMoveHistory pb.MoveHistory
		if err := proto.Unmarshal(b, &pbMoveHistory); err != nil {
			util.LogFatalErr("failed to unmarshal move history", err)
		}
		for _, pbStep := range pbMoveHistory.MoveSteps {
			game, err := data.MapGame(pbStep.Game)
			if err != nil {
				util.LogFatalErr("failed to map game", err)
			}
			fmt.Printf("game with move: %v: %s\n", data.MapPieceMove(pbStep.Move), game.Board.String())
		}
	}
}
