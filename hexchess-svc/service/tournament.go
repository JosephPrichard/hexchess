package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/domain"
	"hexchess-svc/util/enum"
	"hexchess-svc/util/errutil"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsWithUserByTournamentIDRow) domain.Participant {
	return mapLbdUser(sqlc.SelectUserWithEloByIDRow{
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

func mapTourneyMatchFromRow(match sqlc.SelectReplayMatchesByTournamentIDRow) domain.Match {
	var tournamentReplay *domain.TournamentReplay

	// invariant: if replayID is non null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult := enum.Expect(match.Result.ResultEnum, domain.ReplayResultEnums)
		replayCause := enum.Expect(match.Cause.CauseEnum, domain.ReplayCauseEnums)
		replayMode := enum.Expect(match.Mode.ModeEnum, domain.GameModeEnums)

		replay := domain.Replay{
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
		tournamentReplay = &domain.TournamentReplay{
			Replay:    replay,
			RepayView: domain.MakeReplayView(replay),
		}
	}

	return domain.Match{
		Ordering:      match.Ordering,
		GameID:        match.GameID, // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
		WhiteID:       match.WhiteID,
		BlackID:       match.BlackID,
	}
}

func mapTournamentByIdRow(tournament sqlc.SelectTournamentByIDRow) domain.Tournament {
	ruleset := enum.Expect(tournament.Ruleset, domain.TournamentRulesetEnums)
	status := enum.Expect(tournament.Status, domain.TournamentStatusEnums)
	mode := enum.Expect(tournament.Mode, domain.GameModeEnums)

	var maxPlayerCount int
	switch ruleset {
	case domain.TournamentKnockout:
		maxPlayerCount = participantsAtRound(int(tournament.Rounds), 1)
	case domain.TournamentRoundRobin:
		maxPlayerCount = -1
	case domain.TournamentSwiss:
		maxPlayerCount = -1
	}

	return domain.Tournament{
		ID:                 tournament.ID,
		TournamentKey:      tournament.TournamentKey.Bytes,
		Name:               tournament.Name,
		Rounds:             tournament.Rounds,
		WinnerID:           tournament.WinnerID.Int64,
		MaxPlayerCount:     maxPlayerCount,
		CountdownStartedOn: tournament.CountdownStartedOn.Time,
		CountdownStarted:   tournament.CountdownStartedOn.Valid,
		Countdown:          (time.Duration(tournament.Countdown) * time.Millisecond).String(),
		CreatedOn:          tournament.CreatedOn.Time,
		CreatedBy:          tournament.CreatedBy,
		Status:             status,
		Ruleset:            ruleset,
		Mode:               mode,
	}
}

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func (svc *HexchessServices) GetFullTournamentByKey(ctx context.Context, tournamentKey uuid.UUID) (t domain.FullTournament, err error) {
	var tournamentRow sqlc.SelectTournamentByIDRow
	var matchRows []sqlc.SelectReplayMatchesByTournamentIDRow
	var participantRows []sqlc.SelectParticipantsWithUserByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = svc.querier.SelectTournamentByID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select tournament by key [%v]", tournamentKey)
	})

	eg.Go(func() (err error) {
		participantRows, err = svc.querier.SelectParticipantsWithUserByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select participants by tournament key [%v]", tournamentKey)
	})

	eg.Go(func() (err error) {
		matchRows, err = svc.querier.SelectReplayMatchesByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select replay matches by tournament key [%v]", tournamentKey)
	})

	if err := eg.Wait(); err != nil {
		if IsErrNoRows(err) {
			return t, ErrTournamentNotFound
		}
		return t, err
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournamentRow, "matchRows", matchRows, "participantRows", participantRows)

	fullTournament := mapFullTournament(mapFullTournamentArgs{
		tournamentRow:   tournamentRow,
		matchRows:       matchRows,
		participantRows: participantRows,
	})

	participantIDs := make([]int64, 0, len(fullTournament.Participants))
	for _, participant := range fullTournament.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, participantIDs, fullTournament.Mode)
	if err != nil {
		return t, fmt.Errorf("get participants %+v leaderboard rank: %w", participantIDs, err)
	}
	for i := range fullTournament.Participants {
		fullTournament.Participants[i].Rank = userLdbRanksMap[fullTournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "retrieved full tournament", "fullTournament", fullTournament)
	return fullTournament, nil
}

type mapFullTournamentArgs struct {
	tournamentRow   sqlc.SelectTournamentByIDRow
	matchRows       []sqlc.SelectReplayMatchesByTournamentIDRow
	participantRows []sqlc.SelectParticipantsWithUserByTournamentIDRow
}

func mapFullTournament(args mapFullTournamentArgs) domain.FullTournament {
	tournament := mapTournamentByIdRow(args.tournamentRow)

	participants := make([]domain.Participant, 0, len(args.participantRows))
	for _, row := range args.participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]domain.Match, 0, len(args.matchRows))
	for _, row := range args.matchRows {
		matches = append(matches, mapTourneyMatchFromRow(row))
	}

	return domain.FullTournament{Tournament: tournament, Participants: participants, Matches: matches}
}

const NoParticipantSignifier = -1

func (svc *HexchessServices) GetTournaments(ctx context.Context, participantID int64, afterID int64, perPage int32) ([]domain.Tournament, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	var tournaments []domain.Tournament

	if participantID == NoParticipantSignifier {
		tournamentRows, err := svc.querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments after id %d: %w", afterID, err)
		}
		tournaments = mapTournamentRows(tournamentRows, mapSelectTournamentRow)
	} else {
		tournamentRows, err := svc.querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID,
			AfterID: afterID,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments by participant [%d] after id %d: %w", participantID, afterID, err)
		}
		tournaments = mapTournamentRows(tournamentRows, mapTournamentByParticipantRow)
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}

type tournamentRowType interface {
	sqlc.SelectTournamentsRow | sqlc.SelectTournamentsByParticipantRow
}

func mapTournamentByParticipantRow(t sqlc.SelectTournamentsByParticipantRow) domain.Tournament {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapSelectTournamentRow(t sqlc.SelectTournamentsRow) domain.Tournament {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapTournamentRows[Row tournamentRowType](tournamentRows []Row, fn func(tournament Row) domain.Tournament) []domain.Tournament {
	var tournaments []domain.Tournament
	for _, row := range tournamentRows {
		tournaments = append(tournaments, fn(row))
	}
	return tournaments
}

type TournamentInst struct {
	Key       uuid.UUID                `json:"key"`
	Name      string                   `json:"name"`
	Rounds    int32                    `json:"TotalRounds"`
	Mode      domain.GameMode          `json:"mode"`
	Ruleset   domain.TournamentRuleset `json:"ruleset"`
	Countdown time.Duration            `json:"countdown"`
	CreatedOn time.Time                `json:"createdOn"`
	CreatedBy int64                    `json:"createdBy"`
}

const MaxKnockoutTournamentRounds = 5

var ErrInvalidRounds = fmt.Errorf("invalid depth, must be less than %d and larger than 0", MaxKnockoutTournamentRounds)

var InsertionStatus = domain.TournamentLobby.String()

func (svc *HexchessServices) CreateTournamentTx(ctx context.Context, inst TournamentInst) (int64, error) {
	if inst.Ruleset == domain.TournamentKnockout && (inst.Rounds < 1 || inst.Rounds > MaxKnockoutTournamentRounds) {
		return 0, ErrInvalidRounds
	}

	insertionTime := time.Now()
	if inst.CreatedOn.IsZero() {
		inst.CreatedOn = insertionTime
	}

	var rounds int32 // other modes do not calculate the rounds field until tournament starts
	if inst.Ruleset == domain.TournamentKnockout {
		rounds = inst.Rounds
	}

	tournamentID, err := svc.querier.InsertTournament(ctx, sqlc.InsertTournamentParams{
		TournamentKey: pgtype.UUID{Bytes: inst.Key, Valid: true},
		Name:          inst.Name,
		Rounds:        rounds,
		Status:        sqlc.TournamentStatusEnum(InsertionStatus),
		Countdown:     inst.Countdown.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:     inst.CreatedBy,
		UpdatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:          sqlc.ModeEnum(inst.Mode.String()),
		Ruleset:       sqlc.TournamentRulesetEnum(inst.Ruleset.String()),
	})
	if err != nil {
		return 0, fmt.Errorf("insert tournament: %w", err)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentKey", tournamentID, "inst", inst)
	return tournamentID, nil
}

func (svc *HexchessServices) LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error) {
	deletedIDs, err := svc.querier.DeleteTournamentParticipant(ctx, sqlc.DeleteTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		UserID:        userID,
	})
	if err != nil {
		return false, fmt.Errorf("delete participant %d from tournament %s: %w", userID, tournamentKey, err)
	}

	slog.InfoContext(ctx, "deleted tournament participant", "tournamentKey", tournamentKey, "deletedIDs", deletedIDs)
	return len(deletedIDs) > 0, nil
}

// MatchInvariantError is used to wrap errors that violate invariants of the advance tournament matchmaking system so the operation can be aborted rather than retried
type MatchInvariantError struct {
	TournamentKey uuid.UUID
	Err           error
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

type JoinTournamentResult struct {
	TournamentKey uuid.UUID
	Mode          domain.GameMode
}

func (svc *HexchessServices) JoinTournamentTx(ctx context.Context, inst JoinTournamentInst) (JoinTournamentResult, error) {
	var result JoinTournamentResult

	err := svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			result, err = joinTournament(ctx, querier, inst)
			return err
		},
		RetryCount: 5,
	})

	return result, err
}

func joinTournament(ctx context.Context, querier sqlc.Querier, inst JoinTournamentInst) (r JoinTournamentResult, err error) {
	tournamentRow, err := querier.SelectTournamentWithParticipantCountByID(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
	if IsErrNoRows(err) {
		return r, ErrTournamentNotFound
	} else if err != nil {
		return r, fmt.Errorf("select tournament [%s]: %w", inst.TournamentKey, err)
	}

	gameMode := enum.Expect(tournamentRow.Mode, domain.GameModeEnums)
	status := enum.Expect(tournamentRow.Status, domain.TournamentStatusEnums)
	ruleset := enum.Expect(tournamentRow.Ruleset, domain.TournamentRulesetEnums)

	if status != domain.TournamentLobby {
		return r, ErrTournamentNotLobby
	}

	if ruleset == domain.TournamentKnockout {
		// knockout rulesets use the `TotalRounds` field to decide the maximum number of players
		maxKnckoutPlayerCount := int32(participantsAtRound(int(tournamentRow.Rounds), 1))
		isCapacityReached := tournamentRow.ParticipantCount >= maxKnckoutPlayerCount
		if isCapacityReached {
			return r, ErrTooManyParticipants
		}
	}

	if dbErr := querier.InsertTournamentParticipant(ctx, sqlc.InsertTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: inst.TournamentKey, Valid: true},
		UserID:        inst.JoiningUserID,
		JoinedOn:      pgtype.Timestamptz{Time: inst.InsertionTime, Valid: true},
	}); dbErr != nil {
		if svcErr := mapParticipantInsertErr(dbErr); svcErr != nil {
			return r, svcErr
		}
		return r, fmt.Errorf("insert tournament participant [%+v]: %w", inst, dbErr)
	}

	slog.InfoContext(ctx, "joined tournament", "tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "joiningUserID", inst.JoiningUserID)
	return JoinTournamentResult{TournamentKey: inst.TournamentKey, Mode: gameMode}, nil
}

func mapParticipantInsertErr(err error) error {
	return mapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

var (
	ErrTournamentCountdownPermissions   = errors.New("only the creating user can begin the tournament countdown")
	ErrInvalidCountdownTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to begin the countdown")
)

type BeginTourneyCountdown struct {
	TournamentKey uuid.UUID
}

func (svc *HexchessServices) BeginTournamentCountdownTx(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error) {
	var result BeginTourneyCountdown

	err := svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and uses it to decide to begin the countdown, creating a scheduled event E1 and setting the status to S3
		// Between reading S1 and creating E1, another client does the same
		// We will end up with two scheduled events E1 even though the system has an invariant that only one scheduled event may exist
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			result, err = beginTournamentCountdown(ctx, querier, tournamentKey, userID, svc.entropy)
			return err
		},
		RetryCount: 5,
	})

	return result, err
}

func beginTournamentCountdown(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, userID int64, source EntropySource) (b BeginTourneyCountdown, err error) {
	tournamentRow, err := querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	if err != nil {
		return b, fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}

	tournamentStatus := enum.Expect(tournamentRow.Status, domain.TournamentStatusEnums)

	if tournamentStatus != domain.TournamentLobby {
		return b, ErrInvalidCountdownTournamentStatus
	}
	if userID != tournamentRow.CreatedBy {
		return b, ErrTournamentCountdownPermissions
	}

	nextTournamentStatus := domain.TournamentScheduled
	updtTournamentTime := time.Now()

	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey:      pgtype.UUID{Bytes: tournamentKey, Valid: true},
		CountdownStartedOn: pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
		Status:             sqlc.TournamentStatusEnum(nextTournamentStatus.String()),
		UpdatedOn:          pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
	}); err != nil {
		return b, fmt.Errorf("update tournament %s status to %s: %w", tournamentKey, nextTournamentStatus, err)
	}

	scheduledOn := time.Now().Add(time.Duration(tournamentRow.Countdown) * time.Millisecond)

	if err := sendScheduledTournamentEvent(ctx, querier, tournamentKey, scheduledOn); err != nil {
		return b, fmt.Errorf("push scheduled tournament [%s] event: %w", tournamentKey, err)
	}

	slog.InfoContext(ctx, "begin tournament countdown", "tournamentRow", tournamentRow)

	return BeginTourneyCountdown{TournamentKey: tournamentKey}, nil
}

type TournamentStatusAssertionError struct {
	Expected []domain.TournamentStatus
	Got      domain.TournamentStatus
}

func (e TournamentStatusAssertionError) Error() string {
	return fmt.Sprintf("tournament state is invalid: expected %v, got %v", e.Expected, e.Got)
}

func (svc *HexchessServices) AdvanceTournamentTx(ctx context.Context, tournamentKey uuid.UUID) error {
	return svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 selects participationIDs P1 and creates and inserts NextMatches M1
		// Between reading of P1 and insertion of M1, another query deletes a participant to create partcipationID state P2
		// M1 has created and returned games with regards to P1 and may contain NextMatches with players not contained in P2
		// Case 2 (Lost Update):
		// T1 selects the status S1 and uses it to decide that NextMatches M1 can be created, and S2 status should be updated
		// Between reading S1 and insertion of M1, another transaction progresses the state to S3 (such as CANCELLED)
		// NextStatus will be overwritten with the new IN_PROGRESS status (S2), S3 is lost
		// This is because only certain status transitions are legal, progression is linear / forward moving
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			err = advanceTournament(ctx, querier, tournamentKey, svc.entropy)
			return
		},
		RetryCount: 5,
	})
}

var ErrEmptyMatchesTournament = errors.New("tournament has no matches")

var ExpectedAdvanceTournamentStatus = []domain.TournamentStatus{domain.TournamentScheduled, domain.TournamentInProgress}

func advanceTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, entropy EntropySource) error {
	tournamentRow, err := querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	if err != nil {
		return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}

	status := enum.Expect(tournamentRow.Status, domain.TournamentStatusEnums)
	mode := enum.Expect(tournamentRow.Mode, domain.GameModeEnums)
	ruleset := enum.Expect(tournamentRow.Ruleset, domain.TournamentRulesetEnums)

	var matchmaking MatchmakingResult

	// invariant: a tournament can be advanced if it is scheduled or in progress
	switch status {
	case domain.TournamentScheduled:
		participantRows, err := querier.SelectParticipantsForMatchmakingByTournamentID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		if err != nil {
			return fmt.Errorf("select participant ids by tournament key [%s]: %w", tournamentKey, err)
		}

		var participants []FirstMatchParticipant
		for _, row := range participantRows {
			participants = append(participants, FirstMatchParticipant{UserID: row.UserID, Elo: row.Elo})
		}
		matchmaking, err = MakeFirstMatches(FirstMatchmakingRequest{
			Ruleset:      ruleset,
			Mode:         mode,
			Participants: participants,
			TotalRounds:  tournamentRow.Rounds,
		})
		if err != nil {
			return MatchInvariantError{TournamentKey: tournamentKey, Err: err}
		}
	case domain.TournamentInProgress:
		matchRows, err := querier.SelectMatchesByTournamentID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		if err != nil {
			return fmt.Errorf("select participant ids by tournament key %s: %w", tournamentKey, err)
		}
		prevMatches := mapPreviousMatches(matchRows)

		matchmaking, err = DoMatchmaking(MatchmakingRequest{
			Ruleset:     ruleset,
			Matches:     prevMatches,
			GameMode:    mode,
			TotalRounds: tournamentRow.Rounds,
		})
		if err != nil {
			return MatchInvariantError{TournamentKey: tournamentKey, Err: err}
		}
	default:
		return MatchInvariantError{
			TournamentKey: tournamentKey,
			Err:           TournamentStatusAssertionError{Got: status, Expected: ExpectedAdvanceTournamentStatus},
		}
	}

	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		Status:        sqlc.TournamentStatusEnum(matchmaking.NextStatus.String()),
		Rounds:        pgtype.Int4{Int32: matchmaking.TotalRounds, Valid: matchmaking.TotalRounds != 0},
		WinnerID:      pgtype.Int8{Int64: matchmaking.WinnerID, Valid: true},
	}); err != nil {
		return fmt.Errorf("update tournament [%s] status to %+v: %w", tournamentKey, matchmaking, err)
	}

	for len(matchmaking.NextMatches) > 0 {
		var matchInsts []sqlc.BatchInsertTournamentMatchParams

		for _, match := range matchmaking.NextMatches {
			matchInsts = append(matchInsts, sqlc.BatchInsertTournamentMatchParams{
				TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
				Round:         matchmaking.NextMatchRound,
				GameID:        match.GameID,
				WhiteID:       match.WhiteID,
				BlackID:       match.BlackID,
			})
		}

		var batchErrs []error
		querier.BatchInsertTournamentMatch(ctx, matchInsts).Exec(func(i int, err error) {
			if err != nil {
				batchErrs = append(batchErrs, fmt.Errorf("insert tournament match %d: %+v: %w", i, matchInsts[i], err))
			}
		})
		if err := errors.Join(batchErrs...); err != nil {
			return err
		}

		if err := sendCreateTournamentMatchesEvent(ctx, querier, tournamentKey, matchmaking.NextMatches); err != nil {
			return fmt.Errorf("push create tournament matches event [%s]: %w", tournamentKey, err)
		}
	}

	slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey, "matchmakingResult", matchmaking)

	if matchmaking.NextStatus == domain.TournamentFinished {
		slog.InfoContext(ctx, "tournament finished", "tournamentKey", tournamentKey, "winnerID", matchmaking.WinnerID, "tiebreaker", matchmaking.Tiebreaker)
	}

	return nil
}

func mapPreviousMatches(matchRows []sqlc.SelectMatchesByTournamentIDRow) []PrevMatch {
	var matches []PrevMatch

	for _, row := range matchRows {
		result := enum.Expect(row.Result, domain.ReplayResultEnums)
		matches = append(matches, PrevMatch{
			Round:    row.Round,
			WhiteID:  row.WhiteID,
			BlackID:  row.BlackID,
			WhiteElo: domain.DefaultUserElo(row.WhiteElo),
			BlackElo: domain.DefaultUserElo(row.BlackElo),
			Result:   result,
		})
	}

	return matches
}
