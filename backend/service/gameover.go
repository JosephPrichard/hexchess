package svc

import (
	"cmp"
	"context"
	"errors"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgtype"
)

func (services *HexchessServices) InsertFinishedGame(ctx context.Context, finishedGame model.FinishedGame) error {
	var changeSet GameResultChangeSet

	if !finishedGame.WhitePlayer.Present || !finishedGame.BlackPlayer.Present {
		slog.WarnContext(ctx, "both players must be id on a finished game", "gameID", finishedGame.GameID)
		return nil
	}
	whiteID := finishedGame.WhitePlayer.ID
	blackID := finishedGame.BlackPlayer.ID

	moveHistBlob, err := chess.MarshalMoveHistory(finishedGame.Board, finishedGame.Moves)
	if err != nil {
		return serrors.New("marshal move history to s3", err)
	}

	changeSet, err = services.InsertGameResult(ctx, GameResult{
		GameID:       finishedGame.GameID,
		WhiteID:      whiteID,
		BlackID:      blackID,
		ReplayCause:  finishedGame.ReplayCause,
		ReplayResult: finishedGame.ReplayResult,
		ReplayMode:   finishedGame.ReplayMode,
		InsertedTime: time.Now(),
	})
	if err != nil {
		return serrors.New("insert finish game tx", err)
	}
	// note: this happens outside the transaction so we do need to hold a lock for an expended period of time.
	if err := services.UpsertReplayMoveHistories(ctx, changeSet.ReplayID, moveHistBlob); err != nil {
		return serrors.New("insert replay move histories", err)
	}

	// note: replay is selected in a seperate query outside transaction to avoid holding locks. this involves performing more diskIO.
	replay, err := services.GetReplay(ctx, changeSet.ReplayID)
	if err != nil {
		return serrors.New("get replay by id", err, "replayID", changeSet.ReplayID)
	}

	// note: used to keep the cache in sync, this can run outside of a transaction because we have a batch job to recover that payload to the cache.
	if err := services.incrLeaderboard(ctx,
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
	); err != nil {
		return serrors.New("incr leaderboard", err, "changeSet", changeSet)
	}

	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", finishedGame.GameID)

	// note: publishing a tournament event is necessary to trigger advancing the game state *IF* the tournament round is finished
	// this operation is idempotent and safe, if the tournament is not ready to be advanced the operation noops
	tournamentKey, err := services.querier.SelectTournamentByGameID(ctx, finishedGame.GameID)

	if db.IsErrNoRows(err) {
		slog.InfoContext(ctx, "skipping send schedule tournament event", "gameID", finishedGame.GameID)
	} else if err == nil {
		slog.InfoContext(ctx, "publishing schedule tournament event", "gameID", finishedGame.GameID)

		// if two scheduled tournament events run concucurrently, one will advance the tournament and the other will noop
		if err := producers.PublishAdvanceTournamentEvent(ctx, services.querier, tournamentKey.Bytes, time.Now()); err != nil {
			return serrors.New("publish scheduled tournament event", err)
		}
	} else {
		return serrors.New("select tournament by game id", err, "gameID", finishedGame.GameID)
	}

	services.broadcaster.BroadcastGamesEvent(ctx, model.SerializeReplayOutput(finishedGame.GameID, replay), pubsub.Async())

	slog.InfoContext(ctx, "completed inserting finished game event", "key", finishedGame.GameID)
	return nil
}

type GameResult struct {
	GameID       string             `json:"gameId"`
	WhiteID      int64              `json:"whiteId"`
	BlackID      int64              `json:"blackId"`
	ReplayCause  model.ReplayCause  `json:"cause"`
	ReplayResult model.ReplayResult `json:"result"`
	ReplayMode   model.GameMode     `json:"mode"`
	InsertedTime time.Time          `json:"insertedTime"`
	TurnCount    int                `json:"turnCount"`
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

func (services *HexchessServices) InsertGameResult(ctx context.Context, result GameResult) (GameResultChangeSet, error) {
	var changeSet GameResultChangeSet

	err := services.transactor.ExecTx(ctx, db.TxArgs{
		// RepeatableRead is required to prevent the following race conditions
		// Case 1 (Lost Update):
		// T1 selects the user elos E1 and uses calculate and insert user elos E2
		// Between reading E1 and writing E2, another query sets user elos to E3
		// User elos (E3) will be overwritten to E2, the update that progressed E1 to E3 will be lost
		Isolation:  pgx.RepeatableRead,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			mode := sqlc.ModeEnum(result.ReplayMode.String())
			userIDs := []int64{result.WhiteID, result.BlackID}

			existingReplayID, err := querier.SelectReplayIDByGameID(ctx, result.GameID)
			if err == nil {
				changeSet = GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}
				return nil
			} else if !db.IsErrNoRows(err) {
				return serrors.New("select has replay with gameID", err)
			}
			// gameID has not been processed, continue executing the transaction

			// selects are sorted by userID to prevent deadlocks
			slices.SortFunc(userIDs, func(left, right int64) int { return cmp.Compare(left, right) })

			userElos, err := querier.SelectUserModeElosByIDs(ctx, sqlc.SelectUserModeElosByIDsParams{ID: userIDs, Mode: mode})
			if err != nil {
				return serrors.New("select users elo", err, "userIDs", userIDs)
			}

			var updts []sqlc.UpsertUserEloParams
			changeSet, updts = makeInsertGameResultChangeSet(result, userElos)

			// updates are sorted by userID to prevent deadlocks
			slices.SortFunc(updts, func(left, right sqlc.UpsertUserEloParams) int { return cmp.Compare(left.UserID, right.UserID) })

			var batchUpsertErrs []error
			querier.UpsertUserElo(ctx, updts).Exec(func(i int, err error) {
				if err != nil {
					batchUpsertErrs = append(batchUpsertErrs, serrors.New("batch upserting elo", err, "batch", i, "updt", updts[i]))
				}
			})
			if err := errors.Join(batchUpsertErrs...); err != nil {
				return err
			}

			replayInst := sqlc.InsertReplayParams{
				GameID:    result.GameID,
				WhiteID:   pgtype.Int8{Int64: result.WhiteID, Valid: model.IsNonGuestID(result.WhiteID)},
				BlackID:   pgtype.Int8{Int64: result.BlackID, Valid: model.IsNonGuestID(result.BlackID)},
				Result:    sqlc.ResultEnum(result.ReplayResult.String()),
				Cause:     sqlc.CauseEnum(result.ReplayCause.String()),
				Mode:      sqlc.ModeEnum(result.ReplayMode.String()),
				WinElo:    changeSet.WinEloDiff,
				LoseElo:   changeSet.LoseEloDiff,
				WhiteElo:  changeSet.WhiteEloNext,
				BlackElo:  changeSet.BlackEloNext,
				PlayedOn:  pgtype.Timestamptz{Valid: true, Time: result.InsertedTime},
				TurnCount: int32(result.TurnCount),
			}
			replayID, err := querier.InsertReplay(ctx, replayInst)
			if err != nil {
				return serrors.New("insert replay for result", err, "result", result)
			}

			changeSet.ReplayID = replayID

			slog.InfoContext(ctx, "inserted game result", "changeSet", changeSet, "replayInst", replayInst)
			return nil
		},
	})

	return changeSet, err
}

func makeInsertGameResultChangeSet(result GameResult, userModeElos []sqlc.SelectUserModeElosByIDsRow) (GameResultChangeSet, []sqlc.UpsertUserEloParams) {
	changeSet := GameResultChangeSet{}
	var updts []sqlc.UpsertUserEloParams

	if model.IsGuestID(result.WhiteID) || model.IsGuestID(result.BlackID) {
		return changeSet, updts
	}

	mode := sqlc.ModeEnum(result.ReplayMode.String())

	whiteElo, blackElo := model.StartElo, model.StartElo
	for _, row := range userModeElos {
		switch row.UserID {
		case result.WhiteID:
			whiteElo = row.Elo
		case result.BlackID:
			blackElo = row.Elo
		}
	}

	if result.ReplayResult == model.Draw {
		// changeSet is unmodified in draw so this becomes a noop changeSet
		changeSet.WhiteEloNext, changeSet.BlackEloNext = whiteElo, blackElo

		updts = []sqlc.UpsertUserEloParams{
			{UserID: result.WhiteID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: changeSet.WhiteEloNext}, Draws: 1, DefaultElo: model.StartElo},
			{UserID: result.BlackID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: changeSet.BlackEloNext}, Draws: 1, DefaultElo: model.StartElo},
		}
	} else {
		var winElo, loseElo float64

		switch result.ReplayResult {
		case model.WhiteWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.WhiteID, result.BlackID, whiteElo, blackElo
		case model.BlackWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.BlackID, result.WhiteID, blackElo, whiteElo
		default:
		}

		winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))

		switch result.ReplayResult {
		case model.WhiteWin:
			changeSet.WhiteEloNext, changeSet.BlackEloNext = winEloNext, loseEloNext
		case model.BlackWin:
			changeSet.WhiteEloNext, changeSet.BlackEloNext = loseEloNext, winEloNext
		default:
		}

		changeSet.WinEloDiff = winEloNext - winElo
		changeSet.LoseEloDiff = loseEloNext - loseElo

		updts = []sqlc.UpsertUserEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: model.StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: model.StartElo},
		}
	}

	return changeSet, updts
}

func (services *HexchessServices) UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error {
	if err := services.querier.UpsertReplayMoveHistories(ctx, sqlc.UpsertReplayMoveHistoriesParams{
		ReplayID: replayID,
		Data:     data,
	}); err != nil {
		return serrors.New("insert replay move histories", err)
	}
	return nil
}
