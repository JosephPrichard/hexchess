package service

import (
	"context"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/database/mutate"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TournamentUpdater struct {
	// infra
	database.Database
	redis cache.Redis

	// service
	producer UpdateTournamentProducer
}

type UpdateTournamentProducer interface {
	ProduceAdvanceTournament(ctx context.Context, txn pgx.Tx, args producers.AdvanceTournamentArgs) error
}

func NewTournamentUpdater(database database.Database, redis cache.Redis, producer UpdateTournamentProducer) *TournamentUpdater {
	return &TournamentUpdater{Database: database, redis: redis, producer: producer}
}

type TournamentInst struct {
	Key       uuid.UUID               `json:"key"`
	Name      string                  `json:"name"`
	Rounds    int32                   `json:"TotalRounds"`
	Mode      model.GameMode          `json:"mode"`
	Ruleset   model.TournamentRuleset `json:"ruleset"`
	Countdown time.Duration           `json:"countdown"`
	CreatedOn time.Time               `json:"createdOn"`
	CreatedBy int64                   `json:"createdBy"`
}

const MaxKnockoutTournamentRounds = 5

var ErrInvalidRounds = fmt.Errorf("invalid depth, must be less than %d and larger than 0", MaxKnockoutTournamentRounds)

var InsertionStatus = model.TournamentLobby.String()

func (service *TournamentUpdater) CreateTournament(ctx context.Context, inst TournamentInst) (_ int64, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	if inst.Ruleset == model.TournamentKnockout && (inst.Rounds < 1 || inst.Rounds > MaxKnockoutTournamentRounds) {
		return 0, ErrInvalidRounds
	}

	insertionTime := time.Now()
	if inst.CreatedOn.IsZero() {
		inst.CreatedOn = insertionTime
	}

	var rounds int32 // other modes do not calculate the rounds field until tournament starts
	if inst.Ruleset == model.TournamentKnockout {
		rounds = inst.Rounds
	}

	tournamentID, err := service.QuerierMutator.InsertTournament(ctx, mutate.InsertTournamentParams{
		TournamentKey: pgtype.UUID{Bytes: inst.Key, Valid: true},
		Name:          inst.Name,
		Rounds:        rounds,
		Status:        mutate.TournamentStatusEnum(InsertionStatus),
		Countdown:     inst.Countdown.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:     inst.CreatedBy,
		UpdatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:          mutate.ModeEnum(inst.Mode.String()),
		Ruleset:       mutate.TournamentRulesetEnum(inst.Ruleset.String()),
	})
	if err != nil {
		return 0, serrors.New("insert tournament", err, "inst", inst)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentKey", tournamentID, "inst", inst)
	return tournamentID, nil
}

func (service *TournamentUpdater) LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error) {
	deletedIDs, err := service.QuerierMutator.DeleteTournamentParticipant(ctx, mutate.DeleteTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		UserID:        userID,
	})
	if err != nil {
		return false, serrors.New("delete participant from tournament", err, "userID", userID, "key", tournamentKey)
	}

	slog.InfoContext(ctx, "deleted tournament participant", "key", tournamentKey, "deletedIDs", deletedIDs)
	return len(deletedIDs) > 0, nil
}

// MatchInvariantError is used to wrap errors that violate invariants of the advance tournament matchmaking system so the operation can be aborted rather than retried
type MatchInvariantError struct {
	TournamentKey uuid.UUID
	Err           error
}

func NewMatchInvariantError(tournamentKey uuid.UUID, err error) MatchInvariantError {
	return MatchInvariantError{TournamentKey: tournamentKey, Err: err}
}

func (e MatchInvariantError) Error() string {
	return fmt.Sprintf("tournament %s state is invalid: %v", e.TournamentKey, e.Err)
}

type JoinTournamentInst struct {
	TournamentKey uuid.UUID
	JoiningUserID int64
	InsertionTime time.Time
}

type JoinTournamentEvent struct {
	TournamentKey uuid.UUID
	Mode          model.GameMode
}

func (service *TournamentUpdater) JoinTournament(ctx context.Context, inst JoinTournamentInst) (event JoinTournamentEvent, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	err = service.Database.ExecTx(ctx, database.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, _ pgx.Tx, querier database.QuerierMutator) error {
			return joinTournament(ctx, querier, inst, &event)
		},
	})

	return
}

func joinTournament(ctx context.Context, query database.QuerierMutator, inst JoinTournamentInst, event *JoinTournamentEvent) error {
	tournamentRow, err := query.SelectTournamentWithParticipantCountByID(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
	if database.IsErrNoRows(err) {
		return ErrTournamentNotFound
	} else if err != nil {
		return serrors.New("select tournament", err, "tournamentKey", inst.TournamentKey)
	}

	gameMode := enum.Expect(tournamentRow.Mode, model.GameModeEnums)
	status := enum.Expect(tournamentRow.Status, model.TournamentStatusEnums)
	ruleset := enum.Expect(tournamentRow.Ruleset, model.TournamentRulesetEnums)

	if status != model.TournamentLobby {
		return ErrTournamentNotLobby
	}

	if ruleset == model.TournamentKnockout {
		// knockout rulesets use the `TotalRounds` field to decide the maximum number of players
		maxKnockoutPlayerCount := int32(KnockoutParticipantsAtRound(int(tournamentRow.Rounds), 1))
		isCapacityReached := tournamentRow.ParticipantCount >= maxKnockoutPlayerCount
		if isCapacityReached {
			return ErrTooManyParticipants
		}
	}

	if dbErr := query.InsertTournamentParticipant(ctx, mutate.InsertTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: inst.TournamentKey, Valid: true},
		UserID:        inst.JoiningUserID,
		JoinedOn:      pgtype.Timestamptz{Time: inst.InsertionTime, Valid: true},
	}); dbErr != nil {
		if svcErr := mapParticipantInsertErr(dbErr); svcErr != nil {
			return svcErr
		}
		return serrors.New("insert tournament participant", dbErr, "inst", inst)
	}

	slog.InfoContext(ctx, "joined tournament", "tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "joiningUserID", inst.JoiningUserID)
	*event = JoinTournamentEvent{TournamentKey: inst.TournamentKey, Mode: gameMode}
	return nil
}

type BeginTourneyCountdown struct {
	TournamentKey uuid.UUID
}

func (service *TournamentUpdater) BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) (tourneyCountdown BeginTourneyCountdown, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	err = service.Database.ExecTx(ctx, database.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and uses it to decide to begin the countdown, creating a scheduled event E1 and setting the status to S3
		// Between reading S1 and creating E1, another client does the same
		// We will end up with two scheduled events E1 even though the system has an invariant that only one scheduled event may exist
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, txn pgx.Tx, query database.QuerierMutator) error {
			return beginTournamentCountdown(ctx, txn, query, service.producer, tournamentKey, userID, &tourneyCountdown)
		},
	})

	return
}

func beginTournamentCountdown(
	ctx context.Context,
	txn pgx.Tx, query database.QuerierMutator, producer UpdateTournamentProducer,
	tournamentKey uuid.UUID, userID int64, tourneyCountdown *BeginTourneyCountdown,
) error {
	tournamentRow, err := query.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	if err != nil {
		return serrors.New("select tournament by key", err, "key", tournamentKey)
	}

	tournamentStatus := enum.Expect(tournamentRow.Status, model.TournamentStatusEnums)

	if tournamentStatus != model.TournamentLobby {
		return ErrInvalidCountdownTournamentStatus
	}
	if userID != tournamentRow.CreatedBy {
		return ErrTournamentCountdownPermissions
	}

	nextTournamentStatus := model.TournamentScheduled
	updtTournamentTime := time.Now()

	if err := query.UpdateTournamentStatus(ctx, mutate.UpdateTournamentStatusParams{
		TournamentKey:      pgtype.UUID{Bytes: tournamentKey, Valid: true},
		CountdownStartedOn: pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
		Status:             mutate.TournamentStatusEnum(nextTournamentStatus.String()),
		UpdatedOn:          pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
	}); err != nil {
		return serrors.New("update tournament status", err, "key", tournamentKey, "nextTournamentStatus", nextTournamentStatus)
	}

	producerArgs := producers.AdvanceTournamentArgs{
		TournamentKey: tournamentKey,
		ScheduledOn:   time.Now().Add(time.Duration(tournamentRow.Countdown) * time.Second),
	}
	if err := producer.ProduceAdvanceTournament(ctx, txn, producerArgs); err != nil {
		return err
	}

	slog.InfoContext(ctx, "begin tournament countdown", "tournamentRow", tournamentRow)

	*tourneyCountdown = BeginTourneyCountdown{TournamentKey: tournamentKey}
	return nil
}
