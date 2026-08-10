package gameplay

import (
	"cmp"
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/database"
	"hexchess-svc/database/mutator"
	"hexchess-svc/database/query"
	"hexchess-svc/pubsub"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/perf"
	"strconv"

	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgtype"
)

type GameOverService struct {
	database.Database
	redis       cache.Redis
	broadcaster pubsub.Broadcaster
	producer    AdvanceTournamentProducer
}

type ReplayService interface {
	GetReplay(ctx context.Context, replayID int64) (model.FullReplay, error)
	UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error
}

type AdvanceTournamentProducer interface {
	ProduceAdvanceTournament(ctx context.Context, txn pgx.Tx, args producers.AdvanceTournamentArgs) error
}

func NewGameoverService(
	database database.Database,
	redis cache.Redis,
	producer AdvanceTournamentProducer,
	broadcaster pubsub.Broadcaster,
) *GameOverService {
	return &GameOverService{
		Database:    database,
		redis:       redis,
		producer:    producer,
		broadcaster: broadcaster,
	}
}

type FinishedGameResult struct {
	ReplayID int64        `json:"replayId"`
	GameID   model.GameID `json:"gameId"`
}

func (services *GameOverService) InsertFinishedGame(ctx context.Context, finishedGame model.FinishedGame) (FinishedGameResult, error) {
	defer perf.WithContext(ctx).Log()

	insertedTime := time.Now()
	if !finishedGame.InsertedTime.IsZero() {
		insertedTime = time.Now()
	}

	// step 1: persist game result into system of record
	changeSet, err := services.insertGameResult(ctx, GameResult{
		GameID:       finishedGame.GameID,
		WhiteID:      finishedGame.WhitePlayer,
		BlackID:      finishedGame.BlackPlayer,
		ReplayCause:  finishedGame.ReplayCause,
		ReplayResult: finishedGame.ReplayResult,
		ReplayMode:   finishedGame.ReplayMode,
		InsertedTime: insertedTime,
	})
	if err != nil {
		return FinishedGameResult{}, serrors.New("insert finish game tx", err)
	}

	// step 2: persist history of a finished game (move and board data)
	moveHistBlob, err := model.MarshalMoveHistory(chess.MoveHistory{InitialBoard: finishedGame.Board, MoveSeq: finishedGame.Moves})
	if err != nil {
		return FinishedGameResult{}, serrors.New("marshal move histories", err)
	}
	// note(Joseph): this happens outside the transaction, so we do need to hold a lock for an expended period of time.
	if err := services.upsertReplayMoveHistories(ctx, changeSet.ReplayID, moveHistBlob); err != nil {
		return FinishedGameResult{}, serrors.New("insert replay move histories", err)
	}

	// step 3: publishing a tournament event is necessary to trigger advancing the game state *IF* the tournament round is finished
	// this operation is idempotent and safe, if the tournament is not ready to be advanced, the operation noops
	tournamentKey, err := services.Querier().SelectTournamentByGameID(ctx, finishedGame.GameID.String())
	switch {
	case database.IsErrNoRows(err):
		slog.InfoContext(ctx, "skipping send schedule tournament event", "gameID", finishedGame.GameID)
	case err != nil:
		return FinishedGameResult{}, serrors.New("select tournament by game id", err, "gameID", finishedGame.GameID)
	default:
		slog.InfoContext(ctx, "publishing schedule tournament event", "gameID", finishedGame.GameID)
		// if two scheduled tournament events run concurrently, one will advance the tournament and the other will noop
		if err := services.producer.ProduceAdvanceTournament(ctx, nil, producers.AdvanceTournamentArgs{
			TournamentKey: tournamentKey.Bytes,
		}); err != nil {
			return FinishedGameResult{}, serrors.New("publish scheduled tournament event", err)
		}
	}

	// step 4: write through the new updates into the cache, this can run outside a transaction because we have a batch job to recover the update to the cache.
	// note(Joseph): this operation is NOT idempotent, so it MUST be the last operation. once completed, we expect to ack immediately
	if err := services.updateLeaderboard(ctx,
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
		UpdtLbChangeSet{Mode: finishedGame.ReplayMode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
	); err != nil {
		return FinishedGameResult{}, serrors.New("incr leaderboard", err, "changeSet", changeSet)
	}

	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", finishedGame.GameID)

	slog.InfoContext(ctx, "completed inserting finished game event", "key", finishedGame.GameID)
	return FinishedGameResult{ReplayID: changeSet.ReplayID, GameID: finishedGame.GameID}, nil
}

func (services *GameOverService) upsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error {
	if err := services.Mutator().UpsertReplayMoveHistories(ctx, mutator.UpsertReplayMoveHistoriesParams{
		ReplayID: replayID,
		Data:     data,
	}); err != nil {
		return serrors.New("insert replay move histories", err)
	}
	return nil
}

type UpdtLbChangeSet struct {
	Mode    model.GameMode
	ID      int64
	EloDiff float64
}

func (services *GameOverService) updateLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := services.redis.PrimaryClient.Pipeline()

	for _, change := range changes {
		if model.IsGuestID(change.ID) || change.EloDiff == 0 {
			// noop zero value changes
			continue
		}
		modeLbZSet := cache.FmtLeaderboardZSet(change.Mode.String())
		pipe.ZIncrBy(ctx, modeLbZSet, change.EloDiff, strconv.Itoa(int(change.ID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.New("incr leaderboard user", err)
	}

	slog.InfoContext(ctx, "updated leaderboard user", "changes", changes)
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

func (services *GameOverService) insertGameResult(ctx context.Context, result GameResult) (GameResultChangeSet, error) {
	defer perf.WithContext(ctx).Log()

	var changeSet GameResultChangeSet

	err := services.Database.ExecTx(ctx, database.TxArgs{
		// RepeatableRead is required to prevent the following race conditions
		// Case 1 (Lost Update):
		// T1 selects the user elos E1 and calculating and insert user elos E2
		// Between reading E1 and writing E2, another query sets user elos to E3
		// User elos (E3) will be overwritten to E2, the update that progressed E1 to E3 will be lost
		Isolation:  pgx.RepeatableRead,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, _ pgx.Tx, querier database.QuerierMutator) error {
			// step 1: use game ID as an idempotency key to prevent saving the same game result on retry
			userIDs := []int64{result.WhiteID, result.BlackID}

			existingReplayID, err := querier.SelectReplayIDByGameID(ctx, result.GameID.String())
			if err == nil {
				// returning existing state makes this operation idempotent
				changeSet = GameResultChangeSet{ReplayID: existingReplayID, AlreadyExists: true}
				return nil
			} else if !database.IsErrNoRows(err) {
				return serrors.New("select has replay with gameID", err)
			}

			// step 2: select the current state of stats for each game participant
			// selects are sorted by userID to prevent deadlocks
			slices.SortFunc(userIDs, func(left, right int64) int { return cmp.Compare(left, right) })

			userElos, err := querier.SelectUserModeElosByIDs(ctx, query.SelectUserModeElosByIDsParams{
				ID:   userIDs,
				Mode: query.ModeEnum(result.ReplayMode.String()),
			})
			if err != nil {
				return serrors.New("select users elo", err, "userIDs", userIDs)
			}

			// step 3: compute and update the next state of stats for each game participant
			var updts []mutator.UpsertUserEloParams
			changeSet, updts = createInsertGameResultChangeSet(result, userElos)

			// updates are sorted by userID to prevent deadlocks
			slices.SortFunc(updts, func(left, right mutator.UpsertUserEloParams) int { return cmp.Compare(left.UserID, right.UserID) })

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
			replayInst := createInsertReplayParams(result, changeSet)
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

func createInsertReplayParams(result GameResult, changeSet GameResultChangeSet) mutator.InsertReplayParams {
	return mutator.InsertReplayParams{
		GameID:    result.GameID.String(),
		WhiteID:   pgtype.Int8{Int64: result.WhiteID, Valid: model.IsNonGuestID(result.WhiteID)},
		BlackID:   pgtype.Int8{Int64: result.BlackID, Valid: model.IsNonGuestID(result.BlackID)},
		Result:    mutator.ResultEnum(result.ReplayResult.String()),
		Cause:     mutator.CauseEnum(result.ReplayCause.String()),
		Mode:      mutator.ModeEnum(result.ReplayMode.String()),
		WinElo:    changeSet.WinEloDiff,
		LoseElo:   changeSet.LoseEloDiff,
		WhiteElo:  changeSet.WhiteEloNext,
		BlackElo:  changeSet.BlackEloNext,
		PlayedOn:  pgtype.Timestamptz{Valid: true, Time: result.InsertedTime},
		TurnCount: int32(result.TurnCount),
	}
}

func createInsertGameResultChangeSet(result GameResult, userModeElos []query.SelectUserModeElosByIDsRow) (GameResultChangeSet, []mutator.UpsertUserEloParams) {
	changeSet := GameResultChangeSet{}
	var updts []mutator.UpsertUserEloParams

	if model.IsGuestID(result.WhiteID) || model.IsGuestID(result.BlackID) {
		return changeSet, updts
	}

	mode := mutator.ModeEnum(result.ReplayMode.String())

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

		updts = []mutator.UpsertUserEloParams{
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

		winEloNext := winElo + 30*(1.0-user.ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * user.ProbabilityWins(winElo, loseElo))

		switch result.ReplayResult {
		case model.WhiteWin:
			changeSet.WhiteEloNext, changeSet.BlackEloNext = winEloNext, loseEloNext
		case model.BlackWin:
			changeSet.WhiteEloNext, changeSet.BlackEloNext = loseEloNext, winEloNext
		default:
		}

		changeSet.WinEloDiff = winEloNext - winElo
		changeSet.LoseEloDiff = loseEloNext - loseElo

		updts = []mutator.UpsertUserEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: model.StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: model.StartElo},
		}
	}

	return changeSet, updts
}
