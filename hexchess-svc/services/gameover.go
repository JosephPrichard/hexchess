package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

var FinishGameConsumerGroup = "finish_game:consumer"

type FinishGameEvent struct {
	GameID       string           `json:"id"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  PlayerState      `json:"whitePlayer"`
	BlackPlayer  PlayerState      `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayresult"`
	ReplayCause  ReplayCause      `json:"replaycause"`
}

func (svc *Services) PushFinishGameEvent(ctx context.Context, event FinishGameEvent) error {
	streamKey := svc.Redis.FinishGameStreamKey

	bytes, err := MarshalFinishGameEvent(event)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	msgID, err := svc.Redis.Queue.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{
			"data": string(bytes),
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event: %w", err)
	}

	slog.InfoContext(ctx, "pushed finished game event", "id", msgID, "gameID", event.GameID)
	return nil
}

func (svc *Services) insertFinishedGameEvent(ctx context.Context, event FinishGameEvent) error {
	var changeSet GameResultChangeSet

	if !event.WhitePlayer.Present || !event.BlackPlayer.Present {
		return fmt.Errorf("game players must be present on a finished game: %s", event.GameID)
	}
	if event.WhitePlayer.ID < 0 || event.BlackPlayer.ID < 0 {
		slog.WarnContext(ctx, "one or more players for game is a guest, did not write finished game", "gameID", event.GameID, "white", event.WhitePlayer, "black", event.BlackPlayer)
		return nil
	}
	whiteID := event.WhitePlayer.ID
	blackID := event.BlackPlayer.ID

	moveHistBlob, err := chess.MarshalMoveHistory(event.Board, event.Moves)
	if err != nil {
		return fmt.Errorf("marshal move history to s3: %w", err)
	}

	changeSet, err = svc.InsertGameResultTx(ctx, GameResult{
		GameID:       event.GameID,
		WhiteID:      whiteID,
		BlackID:      blackID,
		ReplayCause:  event.ReplayCause,
		ReplayResult: event.ReplayResult,
		ReplayMode:   event.ReplayMode,
		InsertedTime: svc.EntropySource.GetNow(),
		MoveHistBlob: moveHistBlob,
	})
	if err != nil {
		return fmt.Errorf("insert finish game tx: %w", err)
	}

	if !changeSet.IsNoop() {
		slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", event.GameID)

		if err := svc.IncrLeaderboard(ctx,
			UpdtLbChangeSet{Mode: event.ReplayMode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
			UpdtLbChangeSet{Mode: event.ReplayMode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
		); err != nil {
			return fmt.Errorf("incr leaderboard %v: %w", changeSet, err)
		}
	}

	replayEntity, err := svc.GetReplay(ctx, changeSet.ReplayID)
	if err != nil {
		return fmt.Errorf("get replay: %w", err)
	}
	if err := svc.BroadcastGamesEvent(ctx, SerializeReplayOutput(event.GameID, replayEntity)); err != nil {
		return fmt.Errorf("broadcast replay entity output: %w", err)
	}

	slog.InfoContext(ctx, "completed writing finished game", "ID", event.GameID)
	return nil
}

type GameResult struct {
	GameID       string       `json:"gameId"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	ReplayCause  ReplayCause  `json:"cause"`
	ReplayResult ReplayResult `json:"result"`
	ReplayMode   GameMode     `json:"mode"`
	InsertedTime time.Time
	MoveHistBlob []byte
}

type GameResultChangeSet struct {
	ReplayID      int64
	WinID         int64
	LoseID        int64
	WinEloDiff    float64
	LoseEloDiff   float64
	AlreadyExists bool
}

func (cs GameResultChangeSet) IsNoop() bool {
	return cs.LoseEloDiff == 0 && cs.WinEloDiff == 0
}

// InsertGameResultTx executes the insertGameResult operation in a transaction primarily to ensure
func (svc *Services) InsertGameResultTx(ctx context.Context, params GameResult) (GameResultChangeSet, error) {
	var changeSet GameResultChangeSet
	err := svc.DB.ExecTx(ctx, db.TxnArgs{
		QueryFn: func(ctx context.Context, query *sqlc.Queries) (err error) {
			changeSet, err = insertGameResult(ctx, query, params)
			return err
		},
	})
	return changeSet, err
}

// insertGameResult processes a game result, updates player ELO scores, and records the match details in the database.
// It handles win, loss, or draw scenarios and ensures a consistent update order for database operations to prevent deadlocking.
// Returns a GRChangeSet summarizing the changes and any error encountered during processing.
func insertGameResult(ctx context.Context, query *sqlc.Queries, result GameResult) (GameResultChangeSet, error) {
	var changeSet GameResultChangeSet

	ids := []int64{result.WhiteID, result.BlackID}
	slices.SortFunc(ids, func(left, right int64) int { return int(left - right) }) // consistent query order

	mode := sqlc.ModeEnum(result.ReplayMode.String())

	existingReplayID, err := query.SelectReplayIDByGameID(ctx, result.GameID)
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return changeSet, fmt.Errorf("select has replay with gameID: %w", err)
		} else {
			return GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}, nil
		}
	}

	userModeElos, err := query.SelectUserModeElosByIds(ctx, sqlc.SelectUserModeElosByIdsParams{ID: ids, Mode: mode})
	if err != nil {
		return changeSet, fmt.Errorf("select users %+v elo: %w", ids, err)
	}

	whiteElo, blackElo := StartElo, StartElo
	for _, row := range userModeElos {
		switch row.UserID {
		case result.WhiteID:
			whiteElo = row.Elo
		case result.BlackID:
			blackElo = row.Elo
		}
	}

	var whiteEloNext, blackEloNext float64
	var updts []sqlc.UpsertEloParams

	if result.ReplayResult == Draw {
		whiteEloNext, blackEloNext = whiteElo, blackElo

		updts = []sqlc.UpsertEloParams{
			{UserID: result.WhiteID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: whiteEloNext}, Draws: 1, DefaultElo: StartElo},
			{UserID: result.BlackID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: blackEloNext}, Draws: 1, DefaultElo: StartElo},
		}
		// changeSet is unmodified so this becomes a noop changeSet
	} else {
		var winElo, loseElo float64

		switch result.ReplayResult {
		case WhiteWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.WhiteID, result.BlackID, whiteElo, blackElo
		case BlackWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.BlackID, result.WhiteID, blackElo, whiteElo
		default:
		}

		winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))

		switch result.ReplayResult {
		case WhiteWin:
			whiteEloNext, blackEloNext = winEloNext, loseEloNext
		case BlackWin:
			whiteEloNext, blackEloNext = loseEloNext, winEloNext
		default:
		}

		changeSet.WinEloDiff = winEloNext - winElo
		changeSet.LoseEloDiff = loseEloNext - loseElo

		updts = []sqlc.UpsertEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: StartElo},
		}
	}

	slices.SortFunc(updts, func(left, right sqlc.UpsertEloParams) int { return int(left.UserID - right.UserID) }) // consistent update order
	for _, updt := range updts {
		if err := query.UpsertElo(ctx, updt); err != nil {
			return changeSet, fmt.Errorf("upserting elo for user %d: %w", updt.UserID, err)
		}
	}

	replayID, err := insertReplay(ctx, query, ReplayInst{
		GameID:         result.GameID,
		WhiteID:        result.WhiteID,
		BlackID:        result.BlackID,
		Result:         result.ReplayResult,
		Cause:          result.ReplayCause,
		Mode:           result.ReplayMode,
		WinEloDiff:     changeSet.WinEloDiff,
		LoseEloDiff:    changeSet.LoseEloDiff,
		ReplayWhiteElo: whiteEloNext,
		ReplayBlackElo: blackEloNext,
		PlayedOn:       result.InsertedTime,
		MoveHistBlob:   result.MoveHistBlob,
	})
	if err != nil {
		return changeSet, fmt.Errorf("insert replay: %w", err)
	}

	changeSet.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", changeSet)
	return changeSet, nil
}
