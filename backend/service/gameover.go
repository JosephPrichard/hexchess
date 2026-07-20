package svc

import (
	"cmp"
	"context"
	"errors"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgtype"
)

func (services *HexchessServices) InsertFinishedGame(ctx context.Context, finishedGame model.FinishedGame) error {
	// step 1: persist game result into system of record
	changeSet, err := services.InsertGameResult(ctx, GameResult{
		GameID:       finishedGame.GameID,
		WhiteID:      finishedGame.WhitePlayer,
		BlackID:      finishedGame.BlackPlayer,
		ReplayCause:  finishedGame.ReplayCause,
		ReplayResult: finishedGame.ReplayResult,
		ReplayMode:   finishedGame.ReplayMode,
		InsertedTime: time.Now(),
	})
	if err != nil {
		return serrors.New("insert finish game tx", err)
	}

	// step 2: persist history of a finished game (move and board data)
	moveHistBlob, err := model.MarshalMoveHistory(chess.MoveHistory{InitialBoard: finishedGame.Board, MoveSeq: finishedGame.Moves})
	if err != nil {
		return serrors.New("marshal move histories", err)
	}
	// note: this happens outside the transaction, so we do need to hold a lock for an expended period of time.
	if err := services.UpsertReplayMoveHistories(ctx, changeSet.ReplayID, moveHistBlob); err != nil {
		return serrors.New("insert replay move histories", err)
	}

	// step 3: write through the new updates into the cache, this can run outside a transaction because we have a batch job to recover the update to the cache.
	if err := services.incrLeaderboard(ctx,
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
	); err != nil {
		return serrors.New("incr leaderboard", err, "changeSet", changeSet)
	}

	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", finishedGame.GameID)

	// step 4: notify any subscribers of the game that replay has been created (game has ended)
	// note: replay is selected in a separate query outside transaction to avoid holding locks. this involves performing more disk IO.
	replay, err := services.GetReplay(ctx, changeSet.ReplayID)
	if err != nil {
		return serrors.New("get replay by id", err, "replayID", changeSet.ReplayID)
	}
	services.broadcaster.BroadcastGamesEvent(ctx, model.SerializeReplayOutput(model.ReplayGameOutput{GameID: finishedGame.GameID, Replay: replay}))

	// step 5: publishing a tournament event is necessary to trigger advancing the game state *IF* the tournament round is finished
	// this operation is idempotent and safe, if the tournament is not ready to be advanced, the operation noops
	tournamentKey, err := services.querier.SelectTournamentByGameID(ctx, finishedGame.GameID.String())
	if db.IsErrNoRows(err) {
		slog.InfoContext(ctx, "skipping send schedule tournament event", "gameID", finishedGame.GameID)
	} else if err == nil {
		slog.InfoContext(ctx, "publishing schedule tournament event", "gameID", finishedGame.GameID)
		// if two scheduled tournament events run concurrently, one will advance the tournament and the other will noop
		if err := producers.PublishAdvanceTournamentEvent(ctx, services.querier, tournamentKey.Bytes, time.Now()); err != nil {
			return serrors.New("publish scheduled tournament event", err)
		}
	} else {
		return serrors.New("select tournament by game id", err, "gameID", finishedGame.GameID)
	}

	slog.InfoContext(ctx, "completed inserting finished game event", "key", finishedGame.GameID)
	return nil
}

type GameResult struct {
	GameID       model.GameID       `json:"gameId"`
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

	err := services.database.ExecTx(ctx, db.TxArgs[primarydb.Querier]{
		// RepeatableRead is required to prevent the following race conditions
		// Case 1 (Lost Update):
		// T1 selects the user elos E1 and calculating and insert user elos E2
		// Between reading E1 and writing E2, another query sets user elos to E3
		// User elos (E3) will be overwritten to E2, the update that progressed E1 to E3 will be lost
		Isolation:  pgx.RepeatableRead,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, querier primarydb.Querier) error {
			// step 1: use game ID as an idempotency key to prevent persistening the same game result on retry
			mode := primarydb.ModeEnum(result.ReplayMode.String())
			userIDs := []int64{result.WhiteID, result.BlackID}

			existingReplayID, err := querier.SelectReplayIDByGameID(ctx, result.GameID.String())
			if err == nil {
				// returning existing state makes this operation idempotent
				changeSet = GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}
				return nil
			} else if !db.IsErrNoRows(err) {
				return serrors.New("select has replay with gameID", err)
			}

			// step 2: select the current state of stats for each game participant
			// selects are sorted by userID to prevent deadlocks
			slices.SortFunc(userIDs, func(left, right int64) int { return cmp.Compare(left, right) })

			userElos, err := querier.SelectUserModeElosByIDs(ctx, primarydb.SelectUserModeElosByIDsParams{ID: userIDs, Mode: mode})
			if err != nil {
				return serrors.New("select users elo", err, "userIDs", userIDs)
			}

			// step 3: compute and update the next state of stats for each game participant
			var updts []primarydb.UpsertUserEloParams
			changeSet, updts = makeInsertGameResultChangeSet(result, userElos)

			// updates are sorted by userID to prevent deadlocks
			slices.SortFunc(updts, func(left, right primarydb.UpsertUserEloParams) int { return cmp.Compare(left.UserID, right.UserID) })

			var batchUpsertErrs []error
			querier.UpsertUserElo(ctx, updts).Exec(func(i int, err error) {
				if err != nil {
					batchUpsertErrs = append(batchUpsertErrs, serrors.New("batch upserting elo", err, "batch", i, "updt", updts[i]))
				}
			})
			if err := errors.Join(batchUpsertErrs...); err != nil {
				return err
			}

			// step 4: insert the new replay, which acts both the record and the idempotency key for this operation
			replayInst := primarydb.InsertReplayParams{
				GameID:    result.GameID.String(),
				WhiteID:   pgtype.Int8{Int64: result.WhiteID, Valid: model.IsNonGuestID(result.WhiteID)},
				BlackID:   pgtype.Int8{Int64: result.BlackID, Valid: model.IsNonGuestID(result.BlackID)},
				Result:    primarydb.ResultEnum(result.ReplayResult.String()),
				Cause:     primarydb.CauseEnum(result.ReplayCause.String()),
				Mode:      primarydb.ModeEnum(result.ReplayMode.String()),
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

func makeInsertGameResultChangeSet(result GameResult, userModeElos []primarydb.SelectUserModeElosByIDsRow) (GameResultChangeSet, []primarydb.UpsertUserEloParams) {
	changeSet := GameResultChangeSet{}
	var updts []primarydb.UpsertUserEloParams

	if model.IsGuestID(result.WhiteID) || model.IsGuestID(result.BlackID) {
		return changeSet, updts
	}

	mode := primarydb.ModeEnum(result.ReplayMode.String())

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

		updts = []primarydb.UpsertUserEloParams{
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

		updts = []primarydb.UpsertUserEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: model.StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: model.StartElo},
		}
	}

	return changeSet, updts
}

func (services *HexchessServices) UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error {
	if err := services.querier.UpsertReplayMoveHistories(ctx, primarydb.UpsertReplayMoveHistoriesParams{
		ReplayID: replayID,
		Data:     data,
	}); err != nil {
		return serrors.New("insert replay move histories", err)
	}
	return nil
}
