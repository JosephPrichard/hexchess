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
		slog.Warn("both players must be present on a finished game", "gameID", event.GameID)
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
	})
	if err != nil {
		return fmt.Errorf("insert finish game tx: %w", err)
	}
	// note: this happens outside the transaction so we do need to hold a lock for an expended period of time.
	if err = svc.Querier.UpsertReplayMoveHistories(ctx, sqlc.UpsertReplayMoveHistoriesParams{
		ReplayID: changeSet.ReplayID,
		Data:     moveHistBlob,
	}); err != nil {
		return fmt.Errorf("insert replay move histories: %w", err)
	}

	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", event.GameID)

	if err := svc.incrLeaderboard(ctx,
		UpdtLbChangeSet{Mode: event.ReplayMode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
		UpdtLbChangeSet{Mode: event.ReplayMode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
	); err != nil {
		return fmt.Errorf("incr leaderboard %v: %w", changeSet, err)
	}

	replay, err := svc.GetReplay(ctx, changeSet.ReplayID)
	if err != nil {
		return fmt.Errorf("get replay: %w", err)
	}
	if err := svc.BroadcastGamesEvent(ctx, SerializeReplayOutput(event.GameID, replay)); err != nil {
		return fmt.Errorf("broadcast replay entity output: %w", err)
	}

	slog.InfoContext(ctx, "completed inserting finished game event", "ID", event.GameID)
	return nil
}

type GameResult struct {
	GameID       string       `json:"gameId"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	ReplayCause  ReplayCause  `json:"cause"`
	ReplayResult ReplayResult `json:"result"`
	ReplayMode   GameMode     `json:"mode"`
	InsertedTime time.Time    `json:"insertedTime"`
}

type GameResultChangeSet struct {
	ReplayID      int64
	WinID         int64
	LoseID        int64
	WinEloDiff    float64
	LoseEloDiff   float64
	WhiteEloNext  float64
	BlackEloNext  float64
	AlreadyExists bool
}

func (changeSet GameResultChangeSet) IsNoop() bool {
	return changeSet.LoseEloDiff == 0 && changeSet.WinEloDiff == 0
}

// InsertGameResultTx executes the insertGameResult operation in a transaction
func (svc *Services) InsertGameResultTx(ctx context.Context, params GameResult) (GameResultChangeSet, error) {
	var changeSet GameResultChangeSet

	err := svc.DB.ExecTx(ctx, db.Txn{
		QueryFn: func(ctx context.Context, query sqlc.Querier) (err error) {
			changeSet, err = insertGameResult(ctx, query, params)
			return err
		},
	})

	return changeSet, err
}

// insertGameResult processes a game result, updates player ELO scores, and records the match details in the database.
// It handles win, loss, or draw scenarios and ensures a consistent update order for database operations to prevent deadlocking.
// Returns a GRChangeSet summarizing the changes and any error encountered during processing.
func insertGameResult(ctx context.Context, query sqlc.Querier, result GameResult) (GameResultChangeSet, error) {
	mode := sqlc.ModeEnum(result.ReplayMode.String())
	userIDs := []int64{result.WhiteID, result.BlackID}

	existingReplayID, err := query.SelectReplayIDByGameID(ctx, result.GameID)
	if err == nil {
		return GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}, nil
	} else if !IsErrNoRows(err) {
		return GameResultChangeSet{}, fmt.Errorf("select has replay with gameID: %w", err)
	}
	// gameID has not been processed? continue executing the transaction.

	slices.SortFunc(userIDs, func(left, right int64) int { return int(left - right) }) // consistent query order
	userElos, err := query.SelectUserModeElosByIds(ctx, sqlc.SelectUserModeElosByIdsParams{ID: userIDs, Mode: mode})
	if err != nil {
		return GameResultChangeSet{}, fmt.Errorf("select users %+v elo: %w", userIDs, err)
	}

	changeSet, updts := makeInsertGameResultChangeSet(result, userElos)

	slices.SortFunc(updts, func(left, right sqlc.UpsertUserEloParams) int { return int(left.UserID - right.UserID) }) // consistent update order
	var batchUpsertErrs []error
	query.UpsertUserElo(ctx, updts).Exec(func(i int, err error) {
		if err != nil {
			batchUpsertErrs = append(batchUpsertErrs, fmt.Errorf("batch %d: upserting elo for updt %+v: %w", i, updts[i], err))
		}
	})
	if err := errors.Join(batchUpsertErrs...); err != nil {
		return GameResultChangeSet{}, err
	}

	replayInst := mapReplayInst(result, changeSet)
	replayID, err := query.InsertReplay(ctx, replayInst)
	if err != nil {
		return GameResultChangeSet{}, fmt.Errorf("insert replay for result %+v: %w", result, err)
	}

	changeSet.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", changeSet)
	return changeSet, nil
}

func makeInsertGameResultChangeSet(result GameResult, userModeElos []sqlc.SelectUserModeElosByIdsRow) (GameResultChangeSet, []sqlc.UpsertUserEloParams) {
	changeSet := GameResultChangeSet{}
	var updts []sqlc.UpsertUserEloParams

	if IsGuestID(result.WhiteID) || IsGuestID(result.BlackID) {
		return changeSet, updts
	}

	mode := sqlc.ModeEnum(result.ReplayMode.String())

	whiteElo, blackElo := StartElo, StartElo
	for _, row := range userModeElos {
		switch row.UserID {
		case result.WhiteID:
			whiteElo = row.Elo
		case result.BlackID:
			blackElo = row.Elo
		}
	}

	if result.ReplayResult == Draw {
		// changeSet is unmodified in draw so this becomes a noop changeSet
		changeSet.WhiteEloNext, changeSet.BlackEloNext = whiteElo, blackElo

		updts = []sqlc.UpsertUserEloParams{
			{UserID: result.WhiteID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: changeSet.WhiteEloNext}, Draws: 1, DefaultElo: StartElo},
			{UserID: result.BlackID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: changeSet.BlackEloNext}, Draws: 1, DefaultElo: StartElo},
		}
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
			changeSet.WhiteEloNext, changeSet.BlackEloNext = winEloNext, loseEloNext
		case BlackWin:
			changeSet.WhiteEloNext, changeSet.BlackEloNext = loseEloNext, winEloNext
		default:
		}

		changeSet.WinEloDiff = winEloNext - winElo
		changeSet.LoseEloDiff = loseEloNext - loseElo

		updts = []sqlc.UpsertUserEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: StartElo},
		}
	}

	return changeSet, updts
}
