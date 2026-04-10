package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

func (svc *HexchessServices) insertFinishedGameEvent(ctx context.Context, event FinishGameEvent) error {
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
		InsertedTime: svc.entropy.GetNow(),
	})
	if err != nil {
		return fmt.Errorf("insert finish game tx: %w", err)
	}
	// note: this happens outside the transaction so we do need to hold a lock for an expended period of time.
	if err = svc.querier.UpsertReplayMoveHistories(ctx, sqlc.UpsertReplayMoveHistoriesParams{
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
		return fmt.Errorf("incr leaderboard %+v: %w", changeSet, err)
	}

	replay, err := svc.GetReplay(ctx, changeSet.ReplayID)
	if err != nil {
		return fmt.Errorf("get replay by ID %d: %w", changeSet.ReplayID, err)
	}
	if err := svc.BroadcastGamesEvent(ctx, SerializeReplayOutput(event.GameID, replay)); err != nil {
		return fmt.Errorf("broadcast replay entity output: %w", err)
	}

	slog.InfoContext(ctx, "completed inserting finished game event", "key", event.GameID)
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

func (svc *HexchessServices) InsertGameResultTx(ctx context.Context, params GameResult) (GameResultChangeSet, error) {
	var changeSet GameResultChangeSet

	err := svc.db.ExecTx(ctx, db.Tx{
		// RepeatableRead is required to prevent the following race conditions
		// Case 1 (Lost Update):
		// T1 selects the user elos E1 and uses calculate and insert user elos E2
		// Between reading E1 and writing E2, another query sets user elos to E3
		// User elos (E3) will be overwritten to E2, the update that progressed E1 to E3 will be lost
		Isolation: pgx.RepeatableRead,
		QueryFn: func(ctx context.Context, query sqlc.Querier) (err error) {
			changeSet, err = insertGameResult(ctx, query, params)
			return err
		},
		RetryCount: 3,
	})

	return changeSet, err
}

func insertGameResult(ctx context.Context, querier sqlc.Querier, result GameResult) (GameResultChangeSet, error) {
	mode := sqlc.ModeEnum(result.ReplayMode.String())
	userIDs := []int64{result.WhiteID, result.BlackID}

	existingReplayID, err := querier.SelectReplayIDByGameID(ctx, result.GameID)
	if err == nil {
		return GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}, nil
	} else if !IsErrNoRows(err) {
		return GameResultChangeSet{}, fmt.Errorf("select has replay with gameID: %w", err)
	}
	// gameID has not been processed, continue executing the transaction

	userElos, err := querier.SelectUserModeElosByIDs(ctx, sqlc.SelectUserModeElosByIDsParams{ID: userIDs, Mode: mode})
	if err != nil {
		return GameResultChangeSet{}, fmt.Errorf("select users %+v elo: %w", userIDs, err)
	}

	changeSet, updts := makeInsertGameResultChangeSet(result, userElos)

	// updates are sorted by userID to prevent deadlocks
	slices.SortFunc(updts, func(left, right sqlc.UpsertUserEloParams) int { return int(left.UserID - right.UserID) })

	var batchUpsertErrs []error
	querier.UpsertUserElo(ctx, updts).Exec(func(i int, err error) {
		if err != nil {
			batchUpsertErrs = append(batchUpsertErrs, fmt.Errorf("batch %d: upserting elo for updt %+v: %w", i, updts[i], err))
		}
	})
	if err := errors.Join(batchUpsertErrs...); err != nil {
		return GameResultChangeSet{}, err
	}

	replayInst := sqlc.InsertReplayParams{
		GameID:   result.GameID,
		WhiteID:  pgtype.Int8{Int64: result.WhiteID, Valid: IsNonGuestID(result.WhiteID)},
		BlackID:  pgtype.Int8{Int64: result.BlackID, Valid: IsNonGuestID(result.BlackID)},
		Result:   sqlc.ResultEnum(result.ReplayResult.String()),
		Cause:    sqlc.CauseEnum(result.ReplayCause.String()),
		Mode:     sqlc.ModeEnum(result.ReplayMode.String()),
		WinElo:   changeSet.WinEloDiff,
		LoseElo:  changeSet.LoseEloDiff,
		WhiteElo: changeSet.WhiteEloNext,
		BlackElo: changeSet.BlackEloNext,
		PlayedOn: pgtype.Timestamptz{Valid: true, Time: result.InsertedTime},
	}
	replayID, err := querier.InsertReplay(ctx, replayInst)
	if err != nil {
		return GameResultChangeSet{}, fmt.Errorf("insert replay for result %+v: %w", result, err)
	}

	changeSet.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", changeSet)
	return changeSet, nil
}

func makeInsertGameResultChangeSet(result GameResult, userModeElos []sqlc.SelectUserModeElosByIDsRow) (GameResultChangeSet, []sqlc.UpsertUserEloParams) {
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

func (svc *HexchessServices) UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error {
	if err := svc.querier.UpsertReplayMoveHistories(ctx, sqlc.UpsertReplayMoveHistoriesParams{
		ReplayID: replayID,
		Data:     data,
	}); err != nil {
		return fmt.Errorf("insert replay move histories: %w", err)
	}
	return nil
}
