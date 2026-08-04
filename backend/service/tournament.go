package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"
	"hexchess-svc/utils/perf"

	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/serrors"

	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"log/slog"
	"math"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

type TournamentService struct {
	leaderboardService *LeaderboardService

	transactor    db.Transactor
	mutator       mutator.Querier
	querier       query.Querier
	riverProducer producers.RiverProducer
}

func NewTournamentService(
	leaderboardService *LeaderboardService,
	transactor db.Transactor,
	mutator mutator.Querier,
	querier query.Querier,
	riverProducer producers.RiverProducer,
) *TournamentService {
	return &TournamentService{
		leaderboardService: leaderboardService,

		transactor:    transactor,
		mutator:       mutator,
		querier:       querier,
		riverProducer: riverProducer,
	}
}

func (services *TournamentService) GetTournament(ctx context.Context, tournamentKey uuid.UUID) (t model.FullTournament, err error) {
	defer perf.WithContext(ctx).Log()

	var tournamentRow query.SelectTournamentByIDRow
	var matchRows []query.SelectReplayMatchesByTournamentIDRow
	var participantRows []query.SelectParticipantsWithUserByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = services.querier.SelectTournamentByID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select tournament by key", err, "tournamentKey", tournamentKey)
	})

	eg.Go(func() (err error) {
		participantRows, err = services.querier.SelectParticipantsWithUserByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select participants by tournament key", err, "tournamentKey", tournamentKey)
	})

	eg.Go(func() (err error) {
		matchRows, err = services.querier.SelectReplayMatchesByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return serrors.New("select replay matches by tournament key", err, "tournamentKey", tournamentKey)
	})

	if err := eg.Wait(); err != nil {
		if db.IsErrNoRows(err) {
			return t, ErrTournamentNotFound
		}
		return t, err
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournamentRow, "matchRows", matchRows, "participantRows", participantRows)

	tournament := mapFullTournament(mapFullTournamentArgs{
		tournamentRow:   tournamentRow,
		matchRows:       matchRows,
		participantRows: participantRows,
	})

	participantIDs := make([]int64, 0, len(tournament.Participants))
	for _, participant := range tournament.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	userLdbRanksMap, err := services.leaderboardService.GetUsersLeaderboardRank(ctx, participantIDs, tournament.Mode)
	if err != nil {
		return t, serrors.New("get participants leaderboard rank", err, "participantIDs", participantIDs)
	}
	for i := range tournament.Participants {
		tournament.Participants[i].Rank = userLdbRanksMap[tournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "retrieved full tournament", "tournament", tournament)
	return tournament, nil
}

type mapFullTournamentArgs struct {
	tournamentRow   query.SelectTournamentByIDRow
	matchRows       []query.SelectReplayMatchesByTournamentIDRow
	participantRows []query.SelectParticipantsWithUserByTournamentIDRow
}

func maxPlayerCountTournament(ruleset model.TournamentRuleset, rounds int32) int {
	if ruleset == model.TournamentKnockout {
		return KnockoutParticipantsAtRound(int(rounds), 1)
	}
	return -1
}

func mapTournamentByIdRow(tournament query.SelectTournamentByIDRow) model.Tournament {
	ruleset := enum.Expect(tournament.Ruleset, model.TournamentRulesetEnums)
	status := enum.Expect(tournament.Status, model.TournamentStatusEnums)
	mode := enum.Expect(tournament.Mode, model.GameModeEnums)

	maxPlayerCount := maxPlayerCountTournament(ruleset, tournament.Rounds)
	countdown := time.Duration(tournament.Countdown) * time.Millisecond

	return model.Tournament{
		ID:                 tournament.ID,
		TournamentKey:      tournament.TournamentKey.Bytes,
		Name:               tournament.Name,
		Rounds:             tournament.Rounds,
		WinnerID:           tournament.WinnerID.Int64,
		MaxPlayerCount:     maxPlayerCount,
		CountdownStartedOn: tournament.CountdownStartedOn.Time,
		CountdownStarted:   tournament.CountdownStartedOn.Valid,
		Countdown:          countdown.String(),
		CreatedOn:          tournament.CreatedOn.Time,
		CreatedBy:          tournament.CreatedBy,
		Status:             status,
		Ruleset:            ruleset,
		Mode:               mode,
	}
}

func mapTourneyParticipantFromRow(participant query.SelectParticipantsWithUserByTournamentIDRow) model.Participant {
	return mapLbdUser(query.SelectUserWithEloByIDRow{
		ID:         participant.UserID,
		Username:   participant.Username,
		Country:    participant.Country,
		Bio:        participant.Bio,
		JoinedOn:   participant.UserJoinedOn,
		Elo:        participant.Elo,
		HighestElo: participant.HighestElo,
		Wins:       participant.Wins,
		Losses:     participant.Losses,
		Draws:      participant.Draws,
	})
}

func mapTourneyMatchFromRow(match query.SelectReplayMatchesByTournamentIDRow) model.FullMatch {
	var tournamentReplay *model.TournamentReplay

	// invariant: if replayID is non null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult := enum.Expect(match.Result.ResultEnum, model.ReplayResultEnums)
		replayCause := enum.Expect(match.Cause.CauseEnum, model.ReplayCauseEnums)
		replayMode := enum.Expect(match.Mode.ModeEnum, model.GameModeEnums)

		replay := model.Replay{
			ID:          match.ReplayID.Int64,
			WhiteID:     match.WhiteID,
			BlackID:     match.BlackID,
			Result:      replayResult,
			Cause:       replayCause,
			Mode:        replayMode,
			WinEloDiff:  match.WinEloDiff.Float64,
			LoseEloDiff: match.LoseEloDiff.Float64,
			PlayedOn:    match.PlayedOn.Time,
		}
		tournamentReplay = &model.TournamentReplay{
			Replay:          replay,
			ReplayColorElos: model.NewReplayView(replay),
		}
	}

	return model.FullMatch{
		Ordering:      match.Ordering,
		GameID:        model.GameID(match.GameID), // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
		WhiteID:       match.WhiteID,
		BlackID:       match.BlackID,
	}
}

func mapFullTournament(args mapFullTournamentArgs) model.FullTournament {
	tournament := mapTournamentByIdRow(args.tournamentRow)

	participants := make([]model.Participant, 0, len(args.participantRows))
	for _, row := range args.participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]model.FullMatch, 0, len(args.matchRows))
	for _, row := range args.matchRows {
		matches = append(matches, mapTourneyMatchFromRow(row))
	}

	return model.FullTournament{Tournament: tournament, Participants: participants, Matches: matches}
}

func (services *TournamentService) GetTournaments(ctx context.Context, participantID optional.Maybe[int64], afterID optional.Maybe[int64], perPage int32) ([]model.Tournament, error) {
	defer perf.WithContext(ctx).Log()

	if !afterID.Present {
		afterID.Value = int64(math.MaxInt64)
	}

	var tournaments []model.Tournament

	if participantID.Present {
		tournamentRows, err := services.querier.SelectTournamentsByParticipant(ctx, query.SelectTournamentsByParticipantParams{
			UserID:  participantID.Value,
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, serrors.New("select tournaments by participant after id", err, "participantID", participantID, "afterID", afterID)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t query.SelectTournamentsByParticipantRow) model.Tournament {
			return mapTournamentByIdRow(query.SelectTournamentByIDRow(t))
		})
	} else {
		tournamentRows, err := services.querier.SelectTournaments(ctx, query.SelectTournamentsParams{
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, serrors.New("select tournaments after id", err, "afterID", afterID)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t query.SelectTournamentsRow) model.Tournament {
			return mapTournamentByIdRow(query.SelectTournamentByIDRow(t))
		})
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}

func mapTournamentRows[Row interface {
	query.SelectTournamentsRow | query.SelectTournamentsByParticipantRow
}](
	tournamentRows []Row,
	fn func(tournament Row) model.Tournament,
) []model.Tournament {
	var tournaments []model.Tournament
	for _, row := range tournamentRows {
		tournaments = append(tournaments, fn(row))
	}
	return tournaments
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

func (services *TournamentService) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
	defer perf.WithContext(ctx).Log()

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

	tournamentID, err := services.mutator.InsertTournament(ctx, mutator.InsertTournamentParams{
		TournamentKey: pgtype.UUID{Bytes: inst.Key, Valid: true},
		Name:          inst.Name,
		Rounds:        rounds,
		Status:        mutator.TournamentStatusEnum(InsertionStatus),
		Countdown:     inst.Countdown.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:     inst.CreatedBy,
		UpdatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:          mutator.ModeEnum(inst.Mode.String()),
		Ruleset:       mutator.TournamentRulesetEnum(inst.Ruleset.String()),
	})
	if err != nil {
		return 0, serrors.New("insert tournament", err, "inst", inst)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentKey", tournamentID, "inst", inst)
	return tournamentID, nil
}

func (services *TournamentService) LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error) {
	deletedIDs, err := services.mutator.DeleteTournamentParticipant(ctx, mutator.DeleteTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		UserID:        userID,
	})
	if err != nil {
		return false, serrors.New("delete participant from tournament", err, "userID", userID, "tournamentKey", tournamentKey)
	}

	slog.InfoContext(ctx, "deleted tournament participant", "tournamentKey", tournamentKey, "deletedIDs", deletedIDs)
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

var (
	ErrTournamentNotLobby           = fmt.Errorf("tournament is not in lobby status")
	ErrTooManyParticipants          = fmt.Errorf("tournament is full")
	ErrTournamentAlreadyJoined      = fmt.Errorf("user is already a participant of this tournament")
	ErrInvalidTournamentParticipant = fmt.Errorf("tournament participant is invalid")
)

type JoinTournamentInst struct {
	TournamentKey uuid.UUID
	JoiningUserID int64
	InsertionTime time.Time
}

type JoinTournamentEvent struct {
	TournamentKey uuid.UUID
	Mode          model.GameMode
}

func (services *TournamentService) JoinTournament(ctx context.Context, inst JoinTournamentInst) (JoinTournamentEvent, error) {
	defer perf.WithContext(ctx).Log()

	var result JoinTournamentEvent

	err := services.transactor.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, _ pgx.Tx, query db.ReadWriteQuerier) error {
			tournamentRow, err := query.SelectTournamentWithParticipantCountByID(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
			if db.IsErrNoRows(err) {
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

			if dbErr := query.InsertTournamentParticipant(ctx, mutator.InsertTournamentParticipantParams{
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
			result = JoinTournamentEvent{TournamentKey: inst.TournamentKey, Mode: gameMode}
			return nil
		},
	})

	return result, err
}

func mapParticipantInsertErr(err error) error {
	return db.MapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

var (
	ErrTournamentCountdownPermissions   = errors.New("only the creating user can begin the tournament countdown")
	ErrInvalidCountdownTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to begin the countdown")
)

type BeginTourneyCountdown struct {
	TournamentKey uuid.UUID
}

func (services *TournamentService) BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error) {
	defer perf.WithContext(ctx).Log()

	var tourneyCountdown BeginTourneyCountdown

	err := services.transactor.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and uses it to decide to begin the countdown, creating a scheduled event E1 and setting the status to S3
		// Between reading S1 and creating E1, another client does the same
		// We will end up with two scheduled events E1 even though the system has an invariant that only one scheduled event may exist
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, txn pgx.Tx, query db.ReadWriteQuerier) error {
			tournamentRow, err := query.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
			if err != nil {
				return serrors.New("select tournament by key", err, "tournamentKey", tournamentKey)
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

			if err := query.UpdateTournamentStatus(ctx, mutator.UpdateTournamentStatusParams{
				TournamentKey:      pgtype.UUID{Bytes: tournamentKey, Valid: true},
				CountdownStartedOn: pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
				Status:             mutator.TournamentStatusEnum(nextTournamentStatus.String()),
				UpdatedOn:          pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
			}); err != nil {
				return serrors.New("update tournament status", err, "tournamentKey", tournamentKey, "nextTournamentStatus", nextTournamentStatus)
			}

			if err := services.riverProducer.ProduceAdvanceTournament(ctx, txn, producers.AdvanceTournamentArgs{
				TournamentKey: tournamentKey,
				ScheduledOn:   time.Now().Add(time.Duration(tournamentRow.Countdown) * time.Second),
			}); err != nil {
				return serrors.New("publish scheduled tournament event", err, "tournamentKey", tournamentKey)
			}

			slog.InfoContext(ctx, "begin tournament countdown", "tournamentRow", tournamentRow)

			tourneyCountdown = BeginTourneyCountdown{TournamentKey: tournamentKey}
			return nil
		},
	})

	return tourneyCountdown, err
}

type TournamentStatusAssertionError struct {
	Expected []model.TournamentStatus
	Got      model.TournamentStatus
}

func (e TournamentStatusAssertionError) Error() string {
	return fmt.Sprintf("tournament state is invalid: expected %v, got %v", e.Expected, e.Got)
}

var ErrEmptyMatchesTournament = errors.New("tournament has no matches")

var ExpectedAdvanceTournamentStatus = []model.TournamentStatus{model.TournamentScheduled, model.TournamentInProgress}

func (services *TournamentService) AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.MatchCreation, error) {
	defer perf.WithContext(ctx).Log()

	var matchesToCreate []model.MatchCreation

	err := services.transactor.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 selects participationIDs P1 and creates and inserts NextMatches M1
		// Between reading of P1 and insertion of M1, another query deletes a participant to create participationID state P2
		// M1 has created and returned games in regard to P1 and may contain NextMatches with players not contained in P2
		// Case 2 (Lost Update):
		// T1 selects the status S1 and uses it to decide that NextMatches M1 can be created, and S2 status should be updated
		// Between reading S1 and insertion of M1, another transaction progresses the state to S3 (such as CANCELLED)
		// NextStatus will be overwritten with the new IN_PROGRESS status (S2), S3 is lost
		// This is because only certain status transitions are legal, progression is linear / forward moving
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, _ pgx.Tx, query db.ReadWriteQuerier) error {
			previousEvent, err := selectPreviousAdvanceEvent(ctx, query, eventID)
			if err != nil {
				return serrors.New("select previous advance tournament event", err, "eventID", eventID)
			}
			if previousEvent != nil {
				matchesToCreate = previousEvent.Creations
				return nil
			}

			tournament, err := query.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
			if err != nil {
				return serrors.New("select tournament by key", err, "tournamentKey", tournamentKey)
			}
			status := enum.Expect(tournament.Status, model.TournamentStatusEnums)

			var matchmaking MatchmakingOutput

			switch status {
			case model.TournamentScheduled:
				output, err := advanceScheduledTournament(ctx, query, tournament)
				if err != nil {
					return serrors.New("advance scheduled tournament", err)
				}
				matchmaking = output
			case model.TournamentInProgress:
				output, err := advanceInProgressTournament(ctx, query, tournament)
				if err != nil {
					return serrors.New("advance in progress tournament", err)
				}
				matchmaking = output
			default:
				return NewMatchInvariantError(tournamentKey, TournamentStatusAssertionError{Got: status, Expected: ExpectedAdvanceTournamentStatus})
			}

			if err := insertCreatedMatches(ctx, query, eventID, tournamentKey, matchmaking); err != nil {
				return err
			}

			matchesToCreate = matchmaking.NextMatches
			slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey, "matchesToCreate", matchesToCreate)
			return nil
		},
	})

	return matchesToCreate, err
}

func selectPreviousAdvanceEvent(ctx context.Context, query db.ReadWriteQuerier, eventID uuid.UUID) (*model.MatchCreations, error) {
	var matchCreations model.MatchCreations

	eventOutput, err := query.SelectByEventKeyID(ctx, pgtype.UUID{Bytes: eventID, Valid: true})
	switch {
	case db.IsErrNoRows(err):
		slog.InfoContext(ctx, "advance tournament: event id not consumed", "eventID", eventID)
		return nil, nil
	case err != nil:
		return nil, err
	default:
		err := sonic.Unmarshal(eventOutput, &matchCreations)
		return &matchCreations, err
	}
}

func advanceScheduledTournament(ctx context.Context, query db.ReadWriteQuerier, tournament query.SelectTournamentByIDRow) (MatchmakingOutput, error) {
	mode := enum.Expect(tournament.Mode, model.GameModeEnums)
	ruleset := enum.Expect(tournament.Ruleset, model.TournamentRulesetEnums)

	participantRows, err := query.SelectParticipantsForMatchmakingByTournamentID(ctx, tournament.TournamentKey)
	if err != nil {
		return MatchmakingOutput{}, serrors.New("select participant ids by tournament key", err, "tournamentKey", tournament.TournamentKey)
	}

	participants := make([]FirstMatchParticipant, 0, len(participantRows))
	for _, row := range participantRows {
		participants = append(participants, FirstMatchParticipant{UserID: row.UserID, Elo: row.Elo})
	}

	output, err := NewFirstMatches(FirstMatchmakingInput{
		Ruleset:      ruleset,
		Mode:         mode,
		Participants: participants,
		TotalRounds:  tournament.Rounds,
	})
	if err != nil {
		return MatchmakingOutput{}, MatchInvariantError{TournamentKey: tournament.TournamentKey.Bytes, Err: err}
	}
	return output, nil
}

func advanceInProgressTournament(ctx context.Context, query db.ReadWriteQuerier, tournament query.SelectTournamentByIDRow) (MatchmakingOutput, error) {
	mode := enum.Expect(tournament.Mode, model.GameModeEnums)
	ruleset := enum.Expect(tournament.Ruleset, model.TournamentRulesetEnums)

	matchRows, err := query.SelectMatchesByTournamentID(ctx, tournament.TournamentKey)
	if err != nil {
		return MatchmakingOutput{}, serrors.New("select matches by tournament key", err, "tournamentKey", tournament.TournamentKey)
	}

	completedMatches := make([]CompletedPrevMatch, 0, len(matchRows))

	for _, row := range matchRows {
		if !row.Result.Valid {
			// if a match row has no result, it is not completed yet and therefore we cannot perform matchmaking
			return MatchmakingOutput{}, MatchInvariantError{TournamentKey: tournament.TournamentKey.Bytes, Err: ErrMatchRoundCount}
		}
		result := enum.Expect(row.Result.ResultEnum, model.ReplayResultEnums)
		completedMatches = append(completedMatches, CompletedPrevMatch{
			Round:    row.Round,
			WhiteID:  row.WhiteID,
			BlackID:  row.BlackID,
			WhiteElo: model.DefaultUserElo(row.WhiteElo),
			BlackElo: model.DefaultUserElo(row.BlackElo),
			Result:   result,
		})
	}

	output, err := DoMatchmaking(MatchmakingInput{
		Ruleset:     ruleset,
		Matches:     completedMatches,
		GameMode:    mode,
		TotalRounds: tournament.Rounds,
	})
	if err != nil {
		return MatchmakingOutput{}, MatchInvariantError{TournamentKey: tournament.TournamentKey.Bytes, Err: err}
	}
	return output, nil
}

func insertCreatedMatches(
	ctx context.Context,
	query db.ReadWriteQuerier,
	eventID uuid.UUID,
	tournamentKey uuid.UUID,
	matchmaking MatchmakingOutput,
) error {
	if err := insertTournamentMatches(ctx, query, pgtype.UUID{Bytes: tournamentKey, Valid: true}, matchmaking); err != nil {
		return serrors.New("insert tournament matches", err, "tournamentKey", tournamentKey, "matchmaking", matchmaking)
	}

	eventInput, err := sonic.Marshal(model.MatchCreations{Creations: matchmaking.NextMatches})
	if err != nil {
		return err
	}
	if err := query.InsertEventKey(ctx, mutator.InsertEventKeyParams{
		ID:   pgtype.UUID{Bytes: eventID, Valid: true},
		Data: eventInput,
	}); err != nil {
		return serrors.New("insert event with data", err, "eventID", eventID, "matchesToCreate", matchmaking.NextMatches)
	}

	return nil
}

var ErrMatchRoundCount = errors.New("tournament has an invalid completed match count in round")

func insertTournamentMatches(ctx context.Context, query db.ReadWriteQuerier, tournamentKey pgtype.UUID, response MatchmakingOutput) error {
	shouldUpdateWinnerID := response.WinnerID != WinnerIDNone

	if err := query.UpdateTournamentStatus(ctx, mutator.UpdateTournamentStatusParams{
		TournamentKey: tournamentKey,
		Status:        mutator.TournamentStatusEnum(response.NextStatus.String()),
		Rounds:        pgtype.Int4{Int32: response.TotalRounds, Valid: true},
		WinnerID:      pgtype.Int8{Int64: response.WinnerID, Valid: shouldUpdateWinnerID},
	}); err != nil {
		return serrors.New("update tournament status", err, "tournamentKey", tournamentKey, "response", response)
	}

	if len(response.NextMatches) > 0 {
		var matchInsts []mutator.BatchInsertTournamentMatchParams

		for _, match := range response.NextMatches {
			matchInsts = append(matchInsts, mutator.BatchInsertTournamentMatchParams{
				TournamentKey: tournamentKey,
				Round:         response.NextMatchRound,
				GameID:        match.GameID.String(),
				WhiteID:       match.WhiteID,
				BlackID:       match.BlackID,
			})
		}

		var batchErrs []error
		query.BatchInsertTournamentMatch(ctx, matchInsts).Exec(func(i int, err error) {
			if err != nil {
				batchErrs = append(batchErrs, fmt.Errorf("batch insert tournament match %+v: %w", matchInsts[i], err))
			}
		})
		if err := errors.Join(batchErrs...); err != nil {
			return err
		}
	}

	return nil
}
