package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/util/enum"
	"log/slog"
	"math"
	"time"
)

type TournamentDTO struct {
	ID                 int64             `json:"id"`
	TournamentKey      uuid.UUID         `json:"tournamentKey"`
	Name               string            `json:"name"`
	Rounds             int32             `json:"rounds"`
	WinnerID           int64             `json:"winnerId"`
	MaxPlayerCount     int               `json:"maxPlayerCount"`
	CountdownStartedOn time.Time         `json:"countdownStartedOn"`
	CountdownStarted   bool              `json:"isCountdownStarted"`
	Countdown          string            `json:"countdown"`
	CreatedOn          time.Time         `json:"createdOn"`
	CreatedBy          int64             `json:"createdBy"`
	Ruleset            TournamentRuleset `json:"ruleset"`
	Status             TournamentStatus  `json:"status"`
	Mode               GameMode          `json:"mode"`
}

type ParticipantDTO struct {
	TournamentKey uuid.UUID `json:"tournamentKey"`
	JoinedOn      time.Time `json:"joinedOn"`
	LbdUserDTO              // fetches the leaderboard data for the mode the tournament is in.
}

type TournamentReplay struct {
	ReplayDTO
	ReplayViewDTO
}

type MatchDTO struct {
	ID            int64             `json:"id"`
	GameID        string            `json:"gameID"`
	TournamentKey uuid.UUID         `json:"tournamentKey"`
	Round         int32             `json:"round"`
	CreatedOn     time.Time         `json:"createdOn"`
	WhiteID       int64             `json:"whiteId"`
	BlackID       int64             `json:"blackId"`
	Replay        *TournamentReplay `json:"replay"`
}

type FullTournamentDTO struct {
	Participants []ParticipantDTO `json:"participants"`
	Matches      []MatchDTO       `json:"matches"`
	TournamentDTO
}

const MaxKnockoutTournamentRounds = 5

type InvalidDepthError struct {
	actualDepth int32
}

func (e InvalidDepthError) Error() string {
	return fmt.Sprintf("invalid depth: %d, must be less than %d and larger than 0", e.actualDepth, MaxKnockoutTournamentRounds)
}

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsByTournamentIDRow) ParticipantDTO {
	return ParticipantDTO{
		TournamentKey: participant.TournamentKey.Bytes,
		JoinedOn:      participant.TournamentJoinedOn.Time,
		LbdUserDTO: mapLbdUser(sqlc.SelectUserWithEloByIDsRow{
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
		}),
	}
}

func mapTourneyMatchFromRow(match sqlc.SelectReplayMatchesByTournamentIDRow) (MatchDTO, error) {
	var tournamentReplay *TournamentReplay

	// invariant: if replayID is non null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult, resultErr := enum.Parse(match.Result.ResultEnum, ReplayResultEnums)
		replayCause, causeErr := enum.Parse(match.Cause.CauseEnum, ReplayCauseEnums)
		replayMode, modeErr := enum.Parse(match.Mode.ModeEnum, GameModeEnums)

		if err := errors.Join(resultErr, causeErr, modeErr); err != nil {
			return MatchDTO{}, err
		}

		replay := ReplayDTO{
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
		tournamentReplay = &TournamentReplay{
			ReplayDTO:     replay,
			ReplayViewDTO: MakeReplayViewDTO(replay),
		}
	}

	return MatchDTO{
		GameID:        match.GameID, // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
		WhiteID:       match.WhiteID,
		BlackID:       match.BlackID,
	}, nil
}

func mapTournamentByIdRow(tournament sqlc.SelectTournamentByIDRow) (TournamentDTO, error) {
	tournamentStatus, statusErr := enum.Parse(tournament.Status, TournamentStatusEnums)
	gameMode, modeErr := enum.Parse(tournament.Mode, GameModeEnums)

	if err := errors.Join(statusErr, modeErr); err != nil {
		return TournamentDTO{}, err
	}

	return TournamentDTO{
		ID:                 tournament.ID,
		TournamentKey:      tournament.TournamentKey.Bytes,
		Name:               tournament.Name,
		Rounds:             tournament.Rounds,
		WinnerID:           tournament.WinnerID.Int64,
		MaxPlayerCount:     participantsAtRound(int(tournament.Rounds), 1),
		CountdownStartedOn: tournament.CountdownStartedOn.Time,
		CountdownStarted:   tournament.CountdownStartedOn.Valid,
		Countdown:          (time.Duration(tournament.Countdown) * time.Millisecond).String(),
		CreatedOn:          tournament.CreatedOn.Time,
		CreatedBy:          tournament.CreatedBy,
		Status:             tournamentStatus,
		Mode:               gameMode,
	}, nil
}

func (svc *HexchessServices) GetFullTournamentByID(ctx context.Context, tournamentKey uuid.UUID) (t FullTournamentDTO, err error) {
	var tournamentRow sqlc.SelectTournamentByIDRow
	var matchRows []sqlc.SelectReplayMatchesByTournamentIDRow
	var participantRows []sqlc.SelectParticipantsByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	eg.Go(func() (err error) {
		tournamentRow, err = svc.querier.SelectTournamentByID(egCtx, pgTournamentKey)
		if err != nil {
			return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
		}
		return
	})
	eg.Go(func() (err error) {
		participantRows, err = svc.querier.SelectParticipantsByTournamentID(egCtx, pgTournamentKey)
		if err != nil {
			return fmt.Errorf("select participants by tournament key %v: %w", tournamentKey, err)
		}
		return
	})
	eg.Go(func() (err error) {
		matchRows, err = svc.querier.SelectReplayMatchesByTournamentID(egCtx, pgTournamentKey)
		if err != nil {
			return fmt.Errorf("select matches by tournament key %v: %w", tournamentKey, err)
		}
		return
	})

	if err := eg.Wait(); err != nil {
		if IsErrNoRows(err) {
			return t, ErrTournamentNotFound
		}
		return t, err
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournamentRow, "matchRows", matchRows, "participantRows", participantRows)

	fullTournament, err := mapFullTournament(mapFullTournamentArgs{
		tournamentRow:   tournamentRow,
		matchRows:       matchRows,
		participantRows: participantRows,
	})
	if err != nil {
		return t, fmt.Errorf("map full tournament: %w", err)
	}
	participantIDs := make([]int64, 0, len(fullTournament.Participants))
	for _, participant := range fullTournament.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, participantIDs, fullTournament.Mode)
	if err != nil {
		return t, fmt.Errorf("get users %+v leaderboard rank: %w", participantIDs, err)
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
	participantRows []sqlc.SelectParticipantsByTournamentIDRow
}

func mapFullTournament(args mapFullTournamentArgs) (FullTournamentDTO, error) {
	var tournament TournamentDTO
	participants := make([]ParticipantDTO, 0, len(args.participantRows))
	matches := make([]MatchDTO, 0, len(args.matchRows))

	var mapErrors []error

	tournament, err := mapTournamentByIdRow(args.tournamentRow)
	if err != nil {
		mapErrors = append(mapErrors, fmt.Errorf("map tournament row: %w", err))
	}

	for _, row := range args.participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	for i, row := range args.matchRows {
		match, err := mapTourneyMatchFromRow(row)
		if err != nil {
			mapErrors = append(mapErrors, fmt.Errorf("map match %d row: %w", i, err))
			continue
		}
		matches = append(matches, match)
	}

	return FullTournamentDTO{TournamentDTO: tournament, Participants: participants, Matches: matches}, nil
}

func (svc *HexchessServices) GetTournaments(ctx context.Context, participantID int64, afterID int64, perPage int32) ([]TournamentDTO, error) {
	isParticipantProvided := participantID >= 0
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	var tournaments []TournamentDTO
	var err error

	if isParticipantProvided {
		var tournamentRows []sqlc.SelectTournamentsByParticipantRow
		tournamentRows, err = svc.querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID,
			AfterID: afterID,
			PerPage: perPage,
		})
		tournaments, err = mapTournamentRows(tournamentRows, mapParticipantTournamentRow)
	} else {
		var tournamentRows []sqlc.SelectTournamentsRow
		tournamentRows, err = svc.querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID,
			PerPage: perPage,
		})
		tournaments, err = mapTournamentRows(tournamentRows, mapTournamentRow)
	}
	if err != nil {
		return nil, fmt.Errorf("select tournaments by participantID %d, afterID %d: %w", participantID, afterID, err)
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}

type tournamentRowType interface {
	sqlc.SelectTournamentsRow | sqlc.SelectTournamentsByParticipantRow
}

func mapParticipantTournamentRow(t sqlc.SelectTournamentsByParticipantRow) (TournamentDTO, error) {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapTournamentRow(t sqlc.SelectTournamentsRow) (TournamentDTO, error) {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapTournamentRows[Row tournamentRowType](tournamentRows []Row, fn func(tournament Row) (TournamentDTO, error)) ([]TournamentDTO, error) {
	var tournaments []TournamentDTO
	for _, row := range tournamentRows {
		tournament, err := fn(row)
		if err != nil {
			return nil, err
		}
		tournaments = append(tournaments, tournament)
	}
	return tournaments, nil
}

type TournamentInst struct {
	Key       uuid.UUID         `json:"key"`
	Name      string            `json:"name"`
	Rounds    int32             `json:"TotalRounds"`
	Mode      GameMode          `json:"mode"`
	Ruleset   TournamentRuleset `json:"ruleset"`
	Countdown time.Duration     `json:"countdown"`
	CreatedOn time.Time         `json:"createdOn"`
	CreatedBy int64             `json:"createdBy"`
}

var InsertionStatus = TournamentLobby.String()

func (svc *HexchessServices) CreateTournamentTx(ctx context.Context, inst TournamentInst) (int64, error) {
	if inst.Ruleset == TournamentKnockout && (inst.Rounds < 1 || inst.Rounds > MaxKnockoutTournamentRounds) {
		return 0, InvalidDepthError{actualDepth: inst.Rounds}
	}

	insertionTime := svc.entropy.GetNow()
	if inst.CreatedOn.IsZero() {
		inst.CreatedOn = insertionTime
	}

	var rounds int32 // other modes do not calculate the rounds field until tournament starts
	if inst.Ruleset == TournamentKnockout {
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

	slog.InfoContext(ctx, "created tournament", "tournamentKey", tournamentID)
	return tournamentID, nil
}

var (
	ErrTournamentCountdownPermissions   = errors.New("only the creating user can begin the tournament countdown")
	ErrInvalidCountdownTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to begin the countdown")
)

func (svc *HexchessServices) BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) error {
	return svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and uses it to decide to begin the countdown, creating a scheduled event E1 and setting the status to S3
		// Between reading S1 and creating E1, another client does the same
		// We will end up with two scheduled events E1 even though the system has an invariant that only one scheduled event may exist
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			return beginTournamentCountdown(ctx, querier, tournamentKey, userID, svc.entropy.GetNow)
		},
		RetryCount: 3,
	})
}

func beginTournamentCountdown(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, userID int64, getUpdateTime func() time.Time) error {
	pgKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}
	updateTime := getUpdateTime()

	tournamentRow, err := querier.SelectTournamentByID(ctx, pgKey)
	if err != nil {
		return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}

	tournamentStatus, err := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	if err != nil {
		return err
	}

	if tournamentStatus != TournamentLobby {
		return ErrInvalidCountdownTournamentStatus
	}
	if userID != tournamentRow.CreatedBy {
		return ErrTournamentCountdownPermissions
	}

	nextTournamentStatus := TournamentScheduled
	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey:      pgKey,
		CountdownStartedOn: pgtype.Timestamptz{Time: updateTime, Valid: true},
		Status:             sqlc.TournamentStatusEnum(nextTournamentStatus.String()),
		UpdatedOn:          pgtype.Timestamptz{Time: updateTime, Valid: true},
	}); err != nil {
		return fmt.Errorf("update tournament %s status to %s: %w", tournamentKey, nextTournamentStatus, err)
	}

	scheduledOn := updateTime.Add(time.Duration(tournamentRow.Countdown) * time.Millisecond)

	if err := pushScheduledTournamentEvent(ctx, querier, tournamentKey, scheduledOn); err != nil {
		return fmt.Errorf("push scheduled tournament %s event: %w", tournamentKey, err)
	}
	return nil
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

func (svc *HexchessServices) JoinTournamentTx(ctx context.Context, inst JoinTournamentInst) error {
	return svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			return joinTournament(ctx, querier, inst)
		},
		RetryCount: 3,
	})
}

func joinTournament(ctx context.Context, querier sqlc.Querier, inst JoinTournamentInst) error {
	pgKey := pgtype.UUID{Bytes: inst.TournamentKey, Valid: true}

	tournamentRow, err := querier.SelectTournamentWithParticipantCountByID(ctx, pgKey)
	if IsErrNoRows(err) {
		return ErrTournamentNotFound
	} else if err != nil {
		return fmt.Errorf("select tournament %v: %w", inst.TournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)
	if err := errors.Join(statusErr, rulesetErr); err != nil {
		return err
	}

	if status != TournamentLobby {
		return ErrTournamentNotLobby
	}

	if ruleset == TournamentKnockout {
		// knockout rulesets use the `TotalRounds` field to decide the maximum number of players
		maxKnckoutPlayerCount := int32(participantsAtRound(int(tournamentRow.Rounds), 1))
		isCapacityReached := tournamentRow.ParticipantCount >= maxKnckoutPlayerCount
		if isCapacityReached {
			return ErrTooManyParticipants
		}
	}

	if dbErr := querier.InsertTournamentParticipant(ctx, sqlc.InsertTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: inst.TournamentKey, Valid: true},
		UserID:        inst.JoiningUserID,
		JoinedOn:      pgtype.Timestamptz{Time: inst.InsertionTime, Valid: true},
	}); dbErr != nil {
		if svcErr := mapTournamentInsertErr(dbErr); svcErr != nil {
			return svcErr
		}
		return fmt.Errorf("insert tournament participant %+v: %w", inst, dbErr)
	}

	slog.InfoContext(ctx, "joined tournament",
		"tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "userID", inst.JoiningUserID)
	return nil
}

func mapTournamentInsertErr(err error) error {
	return mapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

var ErrInvalidStartTournamentStatus = fmt.Errorf("tournament must be in SCHEDULED status to start")

func (svc *HexchessServices) StartTournamentTx(ctx context.Context, tournamentKey uuid.UUID) error {
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
			return startTournament(ctx, querier, tournamentKey, svc.entropy.GetNow)
		},
		RetryCount: 3,
	})
}

func startTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, getInsertionTime func() time.Time) error {
	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	tournamentRow, err := querier.SelectTournamentByID(ctx, pgTournamentKey)
	if err != nil {
		return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}
	participantRows, err := querier.SelectParticipantsForMatchmakingByTournamentID(ctx, pgTournamentKey)
	if err != nil {
		return fmt.Errorf("select participant ids by tournament key %s: %w", tournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	mode, modeErr := enum.Parse(tournamentRow.Mode, GameModeEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)

	if err := errors.Join(statusErr, modeErr, rulesetErr); err != nil {
		return wrapMatchError(tournamentKey, err)
	}

	if status != TournamentScheduled {
		return wrapMatchError(tournamentKey, ErrInvalidStartTournamentStatus)
	}

	var participants []FirstMatchParticipantDTO
	for _, row := range participantRows {
		participants = append(participants, FirstMatchParticipantDTO{UserID: row.UserID, Elo: row.Elo})
	}
	matchmakingResult, err := MakeFirstMatches(FirstMatchmakingRequest{
		Ruleset:      ruleset,
		Mode:         mode,
		Participants: participants,
		TotalRounds:  tournamentRow.Rounds,
	})
	if err != nil {
		return wrapMatchError(tournamentKey, err)
	}

	if err := putTournamentMatches(ctx, querier, putMatchesInst{
		tournamentKey:        tournamentKey,
		nextTournamentStatus: TournamentInProgress,
		round:                1,
		totalRounds:          matchmakingResult.TotalRounds,
		getInsertionTime:     getInsertionTime,
		matches:              matchmakingResult.Matches,
	}); err != nil {
		return fmt.Errorf("put tournament %s matches: %w", tournamentKey, err)
	}

	slog.InfoContext(ctx, "started tournament", "tournamentKey", tournamentKey, "matchmakingResult", matchmakingResult)
	return nil
}

var ErrInvalidAdvanceTournamentStatus = errors.New("tournament must be in IN_PROGRESS status to start")

func (svc *HexchessServices) AdvanceTournamentTx(ctx context.Context, tournamentKey uuid.UUID) error {
	// (`TournamentMetadata`) does not need isolation within the transaction so it is fetched concurrently and joined within the transaction
	return svc.db.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// ditto from `StartTournamentTx`, same cases apply here
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			err = advanceTournament(ctx, querier, tournamentKey, svc.entropy.GetNow)
			return
		},
		RetryCount: 3,
	})
}

// MatchInvariantError is used to wrap errors that violate invariants of the advance tournament matchmaking system so the operation can be aborted rather than retried
type MatchInvariantError struct {
	TournamentKey uuid.UUID
	Err           error
}

func wrapMatchError(tournamentKey uuid.UUID, err error) MatchInvariantError {
	return MatchInvariantError{TournamentKey: tournamentKey, Err: err}
}

func (e MatchInvariantError) Error() string {
	return fmt.Sprintf("tournament %s state is invalid: %v", e.TournamentKey, e.Err)
}

var ErrEmptyMatchesTournament = errors.New("tournament has no NextMatches")

func advanceTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, getInsertionTime func() time.Time) error {
	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	tournamentRow, err := querier.SelectTournamentByID(ctx, pgTournamentKey)
	if err != nil {
		return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}
	matchRows, err := querier.SelectMatchesByTournamentID(ctx, pgTournamentKey)
	if err != nil {
		return fmt.Errorf("select participant ids by tournament key %s: %w", tournamentKey, err)
	}

	prevMatches, err := mapPreviousMatches(matchRows)
	if err != nil {
		return wrapMatchError(tournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	mode, modeErr := enum.Parse(tournamentRow.Mode, GameModeEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)

	if err := errors.Join(statusErr, modeErr, rulesetErr); err != nil {
		return wrapMatchError(tournamentKey, err)
	}

	// invariant: a tournament should not have been advance if it is not already in progress
	if status != TournamentInProgress {
		return wrapMatchError(tournamentKey, ErrInvalidAdvanceTournamentStatus)
	}

	matchmakingResult, err := DoMatchmaking(MatchmakingRequest{
		Ruleset:     ruleset,
		Matches:     prevMatches,
		GameMode:    mode,
		TotalRounds: tournamentRow.Rounds,
	})
	if err != nil {
		return wrapMatchError(tournamentKey, err)
	}

	if err := putTournamentMatches(ctx, querier, putMatchesInst{
		tournamentKey:        tournamentKey,
		nextTournamentStatus: matchmakingResult.NextStatus,
		round:                matchmakingResult.NextMatchRound,
		winnerID:             matchmakingResult.WinnerID,
		getInsertionTime:     getInsertionTime,
		matches:              matchmakingResult.NextMatches,
	}); err != nil {
		return fmt.Errorf("put tournament %s matches: %w", tournamentKey, err)
	}

	slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey, "matchmakingResult", matchmakingResult)

	if matchmakingResult.NextStatus == TournamentFinished {
		slog.InfoContext(ctx, "tournament finished", "tournamentKey", tournamentKey, "winnerID",
			matchmakingResult.WinnerID, "tiebreaker", matchmakingResult.Tiebreaker)
	}

	return nil
}

func mapPreviousMatches(matchRows []sqlc.SelectMatchesByTournamentIDRow) ([]PrevMatchDTO, error) {
	var matches []PrevMatchDTO
	var parseErrs []error

	for _, row := range matchRows {
		result, err := enum.Parse(row.Result, ReplayResultEnums)
		if err != nil {
			parseErrs = append(parseErrs, err)
			continue
		}
		matches = append(matches, PrevMatchDTO{
			Round:    row.Round,
			WhiteID:  row.WhiteID,
			BlackID:  row.BlackID,
			WhiteElo: defaultElo(row.WhiteElo),
			BlackElo: defaultElo(row.BlackElo),
			Result:   result,
		})
	}

	return matches, errors.Join(parseErrs...)
}

type putMatchesInst struct {
	tournamentKey        uuid.UUID
	nextTournamentStatus TournamentStatus
	totalRounds          int32
	round                int32
	winnerID             int64
	getInsertionTime     func() time.Time
	matches              []CreateTournamentMatchDTO
}

func putTournamentMatches(ctx context.Context, querier sqlc.Querier, inst putMatchesInst) error {
	if len(inst.matches) == 0 {
		return nil
	}

	updateTotalRounds := inst.totalRounds != 0
	updateWinnerID := inst.winnerID != 0

	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey: pgtype.UUID{Bytes: inst.tournamentKey, Valid: true},
		Status:        sqlc.TournamentStatusEnum(inst.nextTournamentStatus.String()),
		Rounds:        pgtype.Int4{Int32: inst.totalRounds, Valid: updateTotalRounds},
		WinnerID:      pgtype.Int8{Int64: inst.winnerID, Valid: updateWinnerID},
		UpdatedOn:     pgtype.Timestamptz{Time: inst.getInsertionTime(), Valid: true},
	}); err != nil {
		return fmt.Errorf("update tournament %s status to %s: %w", inst.tournamentKey, inst.nextTournamentStatus, err)
	}

	var matchInsts []sqlc.BatchInsertTournamentMatchParams
	for _, match := range inst.matches {
		matchInsts = append(matchInsts, sqlc.BatchInsertTournamentMatchParams{
			TournamentKey: pgtype.UUID{Bytes: inst.tournamentKey, Valid: true},
			Round:         inst.round,
			GameID:        match.GameID,
			WhiteID:       match.WhiteID,
			BlackID:       match.BlackID,
			CreatedOn:     pgtype.Timestamptz{Time: inst.getInsertionTime(), Valid: true},
		})
	}

	var batchInsertErrs []error
	querier.BatchInsertTournamentMatch(ctx, matchInsts).Exec(func(i int, err error) {
		if err != nil {
			batchInsertErrs = append(batchInsertErrs, fmt.Errorf("batch %d: inserting tournament matches %+v: %w", i, matchInsts[i], err))
		}
	})
	if err := errors.Join(batchInsertErrs...); err != nil {
		return err
	}

	// create games tournament event message is enqueued atomically
	// this is primarily to ensure if the games are not persisted into redis, the operation can be retried until success
	return pushCreateTournamentMatchesEvent(ctx, querier, inst.tournamentKey, inst.matches)
}
