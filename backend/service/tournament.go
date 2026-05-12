package svc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/enum"
	"hexchess-svc/internal/errutil"
	"hexchess-svc/model"
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func (svc *HexchessServices) GetTournament(ctx context.Context, tournamentKey uuid.UUID) (t model.FullTournament, err error) {
	var tournamentRow sqlc.SelectTournamentByIDRow
	var matchRows []sqlc.SelectReplayMatchesByTournamentIDRow
	var participantRows []sqlc.SelectParticipantsWithUserByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = svc.querier.SelectTournamentByID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select tournament by key %v", tournamentKey)
	})

	eg.Go(func() (err error) {
		participantRows, err = svc.querier.SelectParticipantsWithUserByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select participants by tournament key %v", tournamentKey)
	})

	eg.Go(func() (err error) {
		matchRows, err = svc.querier.SelectReplayMatchesByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select replay matches by tournament key %v", tournamentKey)
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

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, participantIDs, tournament.Mode)
	if err != nil {
		return t, fmt.Errorf("get participants %+v leaderboard rank: %w", participantIDs, err)
	}
	for i := range tournament.Participants {
		tournament.Participants[i].Rank = userLdbRanksMap[tournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "retrieved full tournament", "tournament", tournament)
	return tournament, nil
}

type mapFullTournamentArgs struct {
	tournamentRow   sqlc.SelectTournamentByIDRow
	matchRows       []sqlc.SelectReplayMatchesByTournamentIDRow
	participantRows []sqlc.SelectParticipantsWithUserByTournamentIDRow
}

func maxPlayerCountTournament(ruleset model.TournamentRuleset, rounds int32) int {
	if ruleset == model.TournamentKnockout {
		return knockoutParticipantsAtRound(int(rounds), 1)
	}
	return -1
}

func mapTournamentByIdRow(tournament sqlc.SelectTournamentByIDRow) model.Tournament {
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

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsWithUserByTournamentIDRow) model.Participant {
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

func mapTourneyMatchFromRow(match sqlc.SelectReplayMatchesByTournamentIDRow) model.Match {
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
			Replay:    replay,
			RepayView: model.MakeReplayView(replay),
		}
	}

	return model.Match{
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

func mapFullTournament(args mapFullTournamentArgs) model.FullTournament {
	tournament := mapTournamentByIdRow(args.tournamentRow)

	participants := make([]model.Participant, 0, len(args.participantRows))
	for _, row := range args.participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]model.Match, 0, len(args.matchRows))
	for _, row := range args.matchRows {
		matches = append(matches, mapTourneyMatchFromRow(row))
	}

	return model.FullTournament{Tournament: tournament, Participants: participants, Matches: matches}
}

func (svc *HexchessServices) GetTournaments(ctx context.Context, participantID enum.Optional[int64], afterID enum.Optional[int64], perPage int32) ([]model.Tournament, error) {
	if !afterID.IsPresent {
		afterID.Value = int64(math.MaxInt64)
	}

	var tournaments []model.Tournament

	if participantID.IsPresent {
		tournamentRows, err := svc.querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID.Value,
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments by participant %v after existingID %d: %w", participantID, afterID, err)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t sqlc.SelectTournamentsByParticipantRow) model.Tournament {
			return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
		})
	} else {
		tournamentRows, err := svc.querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments after existingID %d: %w", afterID, err)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t sqlc.SelectTournamentsRow) model.Tournament {
			return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
		})
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}

func mapTournamentRows[Row interface {
	sqlc.SelectTournamentsRow | sqlc.SelectTournamentsByParticipantRow
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

func (svc *HexchessServices) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
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
	Mode          model.GameMode
}

func (svc *HexchessServices) JoinTournament(ctx context.Context, inst JoinTournamentInst) (JoinTournamentResult, error) {
	var result JoinTournamentResult

	err := svc.db.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation:  pgx.Serializable,
		RetryCount: 5,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			tournamentRow, err := querier.SelectTournamentWithParticipantCountByID(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
			if db.IsErrNoRows(err) {
				return ErrTournamentNotFound
			} else if err != nil {
				return fmt.Errorf("select tournament %s: %w", inst.TournamentKey, err)
			}

			gameMode := enum.Expect(tournamentRow.Mode, model.GameModeEnums)
			status := enum.Expect(tournamentRow.Status, model.TournamentStatusEnums)
			ruleset := enum.Expect(tournamentRow.Ruleset, model.TournamentRulesetEnums)

			if status != model.TournamentLobby {
				return ErrTournamentNotLobby
			}

			if ruleset == model.TournamentKnockout {
				// knockout rulesets use the `TotalRounds` field to decide the maximum number of players
				maxKnckoutPlayerCount := int32(knockoutParticipantsAtRound(int(tournamentRow.Rounds), 1))
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
				if svcErr := mapParticipantInsertErr(dbErr); svcErr != nil {
					return svcErr
				}
				return fmt.Errorf("insert tournament participant %+v: %w", inst, dbErr)
			}

			slog.InfoContext(ctx, "joined tournament", "tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "joiningUserID", inst.JoiningUserID)
			result = JoinTournamentResult{TournamentKey: inst.TournamentKey, Mode: gameMode}
			return nil
		},
	})

	return result, err
}

func mapParticipantInsertErr(err error) error {
	return db.MapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

func (svc *HexchessServices) JoinTournamentAndSelectUser(ctx context.Context, inst JoinTournamentInst) (model.LbdUser, error) {
	result, err := svc.JoinTournament(ctx, inst)
	if err != nil {
		return model.LbdUser{}, fmt.Errorf("join tournament: %w", err)
	}
	lbdUser, err := svc.GetLeaderboardUser(ctx, inst.JoiningUserID, result.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user", "Err", err)
	}
	return lbdUser, nil
}

var (
	ErrTournamentCountdownPermissions   = errors.New("only the creating user can begin the tournament countdown")
	ErrInvalidCountdownTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to begin the countdown")
)

type BeginTourneyCountdown struct {
	TournamentKey uuid.UUID
}

func (svc *HexchessServices) BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error) {
	var tourneyCountdown BeginTourneyCountdown

	err := svc.db.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and uses it to decide to begin the countdown, creating a scheduled event E1 and setting the status to S3
		// Between reading S1 and creating E1, another client does the same
		// We will end up with two scheduled events E1 even though the system has an invariant that only one scheduled event may exist
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			tournamentRow, err := querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
			if err != nil {
				return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
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

			if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
				TournamentKey:      pgtype.UUID{Bytes: tournamentKey, Valid: true},
				CountdownStartedOn: pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
				Status:             sqlc.TournamentStatusEnum(nextTournamentStatus.String()),
				UpdatedOn:          pgtype.Timestamptz{Time: updtTournamentTime, Valid: true},
			}); err != nil {
				return fmt.Errorf("update tournament %s status to %s: %w", tournamentKey, nextTournamentStatus, err)
			}

			scheduledOn := time.Now().Add(time.Duration(tournamentRow.Countdown) * time.Millisecond)

			if err := sendScheduledTournamentEvent(ctx, querier, tournamentKey, scheduledOn); err != nil {
				return fmt.Errorf("push scheduled tournament %s event: %w", tournamentKey, err)
			}

			slog.InfoContext(ctx, "begin tournament countdown", "tournamentRow", tournamentRow)

			tourneyCountdown = BeginTourneyCountdown{TournamentKey: tournamentKey}
			return nil
		},
		RetryCount: 5,
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

func (svc *HexchessServices) AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID) error {
	return svc.db.ExecTx(ctx, db.TxArgs{
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
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			tournamentRow, err := querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
			if err != nil {
				return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
			}

			status := enum.Expect(tournamentRow.Status, model.TournamentStatusEnums)

			var response MatchmakingResponse

			switch status {
			case model.TournamentScheduled:
				if response, err = matchmakeScheduledTournament(ctx, querier, tournamentRow); err != nil {
					return err
				}
			case model.TournamentInProgress:
				if response, err = matchmakeInProgressTournament(ctx, querier, tournamentRow); err != nil {
					return err
				}
			default:
				return MatchInvariantError{
					TournamentKey: tournamentKey,
					Err:           TournamentStatusAssertionError{Got: status, Expected: ExpectedAdvanceTournamentStatus},
				}
			}

			if err := insertTournamentMatches(ctx, querier, tournamentKey, response); err != nil {
				return err
			}

			slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey, "matchmakingResult", response)
			return nil
		},
		RetryCount: 5,
	})
}

func matchmakeScheduledTournament(ctx context.Context, querier sqlc.Querier, tournamentRow sqlc.SelectTournamentByIDRow) (MatchmakingResponse, error) {
	mode := enum.Expect(tournamentRow.Mode, model.GameModeEnums)
	ruleset := enum.Expect(tournamentRow.Ruleset, model.TournamentRulesetEnums)

	participantRows, err := querier.SelectParticipantsForMatchmakingByTournamentID(ctx, tournamentRow.TournamentKey)
	if err != nil {
		return MatchmakingResponse{}, fmt.Errorf("select participant ids by tournament key %s: %w", tournamentRow.TournamentKey, err)
	}

	var participants []FirstMatchParticipant
	for _, row := range participantRows {
		participants = append(participants, FirstMatchParticipant{UserID: row.UserID, Elo: row.Elo})
	}

	response, err := MakeFirstMatches(FirstMatchmakingRequest{
		Ruleset:      ruleset,
		Mode:         mode,
		Participants: participants,
		TotalRounds:  tournamentRow.Rounds,
	})
	if err != nil {
		return MatchmakingResponse{}, MatchInvariantError{TournamentKey: tournamentRow.TournamentKey.Bytes, Err: err}
	}

	return response, nil
}

var ErrMatchRoundCount = errors.New("tournament has an invalid completed match count in round")

func matchmakeInProgressTournament(ctx context.Context, querier sqlc.Querier, tournamentRow sqlc.SelectTournamentByIDRow) (MatchmakingResponse, error) {
	mode := enum.Expect(tournamentRow.Mode, model.GameModeEnums)
	ruleset := enum.Expect(tournamentRow.Ruleset, model.TournamentRulesetEnums)

	matchRows, err := querier.SelectMatchesByTournamentID(ctx, tournamentRow.TournamentKey)
	if err != nil {
		return MatchmakingResponse{}, fmt.Errorf("select participant ids by tournament key %s: %w", tournamentRow.TournamentKey, err)
	}

	var completedMatches []CompletedPrevMatch

	for _, row := range matchRows {
		if !row.Result.Valid {
			// if a match row has no result, it is not completed yet and therefore we cannot perform matchmaking
			return MatchmakingResponse{}, MatchInvariantError{TournamentKey: tournamentRow.TournamentKey.Bytes, Err: ErrMatchRoundCount}
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

	response, err := DoMatchmaking(MatchmakingRequest{
		Ruleset:     ruleset,
		Matches:     completedMatches,
		GameMode:    mode,
		TotalRounds: tournamentRow.Rounds,
	})
	if err != nil {
		return MatchmakingResponse{}, MatchInvariantError{TournamentKey: tournamentRow.TournamentKey.Bytes, Err: err}
	}

	return response, nil
}

func insertTournamentMatches(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, response MatchmakingResponse) error {
	shouldUpdateWinnerID := response.WinnerID != WinnerIDNone

	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		Status:        sqlc.TournamentStatusEnum(response.NextStatus.String()),
		Rounds:        pgtype.Int4{Int32: response.TotalRounds, Valid: true},
		WinnerID:      pgtype.Int8{Int64: response.WinnerID, Valid: shouldUpdateWinnerID},
	}); err != nil {
		return fmt.Errorf("update tournament %s status to %+v: %w", tournamentKey, response, err)
	}

	if len(response.NextMatches) > 0 {
		var matchInsts []sqlc.BatchInsertTournamentMatchParams

		for _, match := range response.NextMatches {
			matchInsts = append(matchInsts, sqlc.BatchInsertTournamentMatchParams{
				TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
				Round:         response.NextMatchRound,
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

		if err := sendCreateTourneytMatchesEvent(ctx, querier, tournamentKey, response.NextMatches); err != nil {
			return fmt.Errorf("send create tournament %s matches event: %w", tournamentKey, err)
		}
	}

	return nil
}

type FirstMatchParticipant struct {
	UserID int64
	Elo    pgtype.Float8
}

type FirstMatchmakingRequest struct {
	Ruleset      model.TournamentRuleset
	Mode         model.GameMode
	Participants []FirstMatchParticipant
	TotalRounds  int32
}

type MatchmakingResponse struct {
	NextMatches    []model.TournamentMatchCreation
	TotalRounds    int32 // RoundRobin and Swiss calculate total rounds during matchmaking rather than using already existing rounds to validate
	NextStatus     model.TournamentStatus
	NextMatchRound int32
	WinnerID       int64
	Tiebreaker     TieBreakerKind
}

type TieBreakerKind int

const (
	TiebreakerNone TieBreakerKind = iota
	TiebreakerByElo
	TiebreakerBySonnebornBerger
)

type CompletedPrevMatch struct {
	Round    int32
	WhiteID  int64
	BlackID  int64
	WhiteElo float64
	BlackElo float64
	Result   model.ReplayResult
}

func getKnockoutWinnerID(match CompletedPrevMatch) (int64, TieBreakerKind) {
	switch match.Result {
	case model.WhiteWin:
		return match.WhiteID, TiebreakerNone
	case model.BlackWin:
		return match.BlackID, TiebreakerNone
	// tiebreaker for draw uses the player with the highest elo, with white as a last case scenario
	case model.Draw:
		if match.WhiteElo >= match.BlackElo {
			return match.WhiteID, TiebreakerByElo
		} else {
			return match.BlackID, TiebreakerByElo
		}
	default:
		panic(fmt.Sprintf("unknown match result %s", match.Result))
	}
}

func withoutTiebreaker(u int64, _ TieBreakerKind) int64 {
	return u
}

func calcRoundRobinTournamentRounds(participantCount int) int32 {
	return int32((participantCount * (participantCount - 1)) / 2)
}

func calcSwissTournamentRounds(participantCount int) int32 {
	return int32(math.Log2(float64(participantCount)))
}

// knockoutParticipantsAtRound returns the number of elements at a given depth.
// Each node holds 2 elements. At depth d, there are 2^(N-d) nodes.
func knockoutParticipantsAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth + 1) }

// knockoutMatchesAtRound returns the number of nodes at a given depth.
// Root (depth N) has 1 node; each level down doubles the count.
func knockoutMatchesAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth) }

type MatchCountErrKind int

const (
	ParticipantCountErrKind = iota
	ParticipantParityErrKind
	MatchParityErrKind
)

type MatchCountError struct {
	Kind      MatchCountErrKind
	WantCount int
	GotCount  int
}

func (e MatchCountError) Error() string {
	switch e.Kind {
	case ParticipantCountErrKind:
		return fmt.Sprintf("tournament requires %v participants, got: %d", e.WantCount, e.GotCount)
	case ParticipantParityErrKind:
		return fmt.Sprintf("tournament requires an even number of participants, got: %d", e.GotCount)
	case MatchParityErrKind:
		return fmt.Sprintf("tournament requires an even number of matches, got: %d", e.GotCount)
	default:
		return fmt.Sprintf("unknown match participant count error kind %d", e.Kind)
	}
}

func makeMatchesLinearly(participants []FirstMatchParticipant, gameMode model.GameMode) []model.TournamentMatchCreation {
	// invariant: participant count is always even (`elementsAtFirstDepth` always returns even)
	if len(participants)%2 != 0 {
		// assert rather than return an error because this property is statically encoded into the `ElementsAtFirstDepth` algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participants))
	}
	var matches []model.TournamentMatchCreation
	for i := 0; i+1 < len(participants); i += 2 {
		matches = append(matches, model.TournamentMatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participants[i].UserID,
			BlackID:  participants[i+1].UserID,
		})
	}
	return matches
}

func makeMatchesCrissCrossElos(participants []FirstMatchParticipant, gameMode model.GameMode) []model.TournamentMatchCreation {
	// sort participants by elo.
	slices.SortFunc(participants, func(a, b FirstMatchParticipant) int {
		if n := cmp.Compare(model.DefaultUserElo(b.Elo), model.DefaultUserElo(a.Elo)); n != 0 {
			return n
		}
		return cmp.Compare(a.UserID, b.UserID)
	})

	var matches []model.TournamentMatchCreation
	low := 0
	high := len(participants) - 1
	for low < high {
		matches = append(matches, model.TournamentMatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participants[low].UserID,
			BlackID:  participants[high].UserID,
		})
		low++
		high--
	}

	// invariant: always converges on a different player (so each player gets a match)
	if low == high {
		// assert rather than return an error because this property is statically encoded into the this algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participants))
	}
	return matches
}

func MakeFirstMatches(request FirstMatchmakingRequest) (MatchmakingResponse, error) {
	participantCount := len(request.Participants)

	var matches []model.TournamentMatchCreation
	totalRounds := request.TotalRounds

	switch request.Ruleset {
	case model.TournamentKnockout:
		// invariant: matches are devided by two each time and stop at 1, we need to start at the expected power of 2
		wantRoundCount := knockoutParticipantsAtRound(int(totalRounds), 1)
		if participantCount != wantRoundCount {
			return MatchmakingResponse{}, MatchCountError{Kind: ParticipantCountErrKind, WantCount: wantRoundCount, GotCount: participantCount}
		}
	case model.TournamentRoundRobin, model.TournamentSwiss:
		// invariant: as long as we can match each player with another player, we can start the tournament
		if participantCount%2 != 0 {
			return MatchmakingResponse{}, MatchCountError{Kind: ParticipantParityErrKind, GotCount: participantCount}
		}
	}

	switch request.Ruleset {
	case model.TournamentKnockout:
		matches = makeMatchesLinearly(request.Participants, request.Mode)

		wantRoundCount := knockoutMatchesAtRound(int(totalRounds), 1)
		if len(matches) != wantRoundCount {
			panic(fmt.Sprintf("expected %d matches, got %d", wantRoundCount, len(matches)))
		}
	case model.TournamentRoundRobin:
		matches = makeMatchesLinearly(request.Participants, request.Mode)
		totalRounds = calcRoundRobinTournamentRounds(participantCount)
	case model.TournamentSwiss:
		// a swiss tournament matches the worst players with the best players
		matches = makeMatchesCrissCrossElos(request.Participants, request.Mode)
		totalRounds = calcSwissTournamentRounds(participantCount)
	default:
		return MatchmakingResponse{}, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	return MatchmakingResponse{NextMatches: matches, NextStatus: model.TournamentInProgress, NextMatchRound: 1, TotalRounds: totalRounds}, nil
}

type MatchmakingRequest struct {
	Ruleset     model.TournamentRuleset
	Matches     []CompletedPrevMatch
	GameMode    model.GameMode
	TotalRounds int32
}

func getPrevRoundMatches(matches []CompletedPrevMatch) []CompletedPrevMatch {
	prevMatchRound := matches[len(matches)-1].Round
	var prevRoundMatches []CompletedPrevMatch
	for _, match := range matches {
		if match.Round == prevMatchRound {
			prevRoundMatches = append(prevRoundMatches, match)
		}
	}
	return prevRoundMatches
}

const WinnerIDNone = int64(0)

func DoMatchmaking(request MatchmakingRequest) (MatchmakingResponse, error) {
	// validation: a tournament must have matches to do matchmaking
	if len(request.Matches) == 0 {
		return MatchmakingResponse{}, ErrEmptyMatchesTournament
	}

	gameMode := request.GameMode
	allMatches := request.Matches

	var nextMatches []model.TournamentMatchCreation

	switch request.Ruleset {
	case model.TournamentKnockout:
		if len(allMatches)%2 != 0 {
			return MatchmakingResponse{}, MatchCountError{Kind: MatchParityErrKind, GotCount: len(allMatches)}
		}
		nextMatches = DoKnockoutMatchmaking(allMatches, gameMode)
	case model.TournamentRoundRobin:
		nextMatches = DoRoundRobinMatchmaking(allMatches, gameMode)
	case model.TournamentSwiss:
		nextMatches = DoSwissMatchmaking(allMatches, gameMode)
	default:
		return MatchmakingResponse{}, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	prevMatchRound := allMatches[len(allMatches)-1].Round
	nextMatchRound := prevMatchRound + 1

	nextStatus := model.TournamentInProgress
	winnerID := WinnerIDNone // defaults to no winner
	tiebreaker := TiebreakerNone

	if nextMatchRound > request.TotalRounds {
		nextStatus = model.TournamentFinished
		winnerID, tiebreaker = findTournamentWinner(request.Ruleset, request.Matches)
	}

	return MatchmakingResponse{
		NextMatches:    nextMatches,
		NextStatus:     nextStatus,
		NextMatchRound: nextMatchRound,
		TotalRounds:    request.TotalRounds,
		WinnerID:       winnerID,
		Tiebreaker:     tiebreaker,
	}, nil
}

func DoKnockoutMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.TournamentMatchCreation {
	var nextMatches []model.TournamentMatchCreation

	prevRoundMatches := getPrevRoundMatches(allMatches)

	for i := 0; i+1 < len(prevRoundMatches); i += 2 {
		matchOne := prevRoundMatches[i]
		matchTwo := prevRoundMatches[i+1]
		nextMatches = append(nextMatches, model.TournamentMatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  withoutTiebreaker(getKnockoutWinnerID(matchOne)),
			BlackID:  withoutTiebreaker(getKnockoutWinnerID(matchTwo)),
		})
	}

	if len(nextMatches) != len(prevRoundMatches)/2 {
		panic(fmt.Sprintf("expected %d next matches, got %d", len(prevRoundMatches)/2, len(nextMatches)))
	}

	return nextMatches
}

func DoRoundRobinMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.TournamentMatchCreation {
	var nextMatches []model.TournamentMatchCreation

	prevRoundMatches := getPrevRoundMatches(allMatches)

	for i := range len(prevRoundMatches) {
		prevMatch := prevRoundMatches[i]

		var nextWhiteID int64
		var nextBlackID int64

		if i == 0 {
			// case (first match): nextWhiteID acts as an 'anchor' (only player that does not change)
			nextWhiteID = prevMatch.WhiteID
			nextBlackID = prevRoundMatches[len(prevRoundMatches)-1].BlackID
		} else {
			// case (other match): take nextWhiteID from previous match, shift previous nextWhiteID into next nextBlackID
			nextWhiteID = prevRoundMatches[i-1].BlackID
			nextBlackID = prevMatch.WhiteID
		}

		nextMatches = append(nextMatches, model.TournamentMatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  nextWhiteID,
			BlackID:  nextBlackID,
		})
	}

	return nextMatches
}

func DoSwissMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.TournamentMatchCreation {
	var nextMatches []model.TournamentMatchCreation

	swissScoresTable := makeSwissTable(allMatches)

	// collect and reverse sort match participants by swiss score
	var participantIDs []int64
	for userID, _ := range swissScoresTable {
		participantIDs = append(participantIDs, userID)
	}
	slices.SortFunc(participantIDs, func(a, b int64) int {
		return cmp.Compare(swissScoresTable[b], swissScoresTable[a])
	})

	for i := 0; i+1 < len(participantIDs); i += 2 {
		nextMatches = append(nextMatches, model.TournamentMatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participantIDs[i],
			BlackID:  participantIDs[i+1],
		})
	}

	return nextMatches
}

func findScoringTableWinners[Score cmp.Ordered](scoreTable map[int64]Score, skip func(int64) bool) []int64 {
	var highestScore Score
	var winnerIDs []int64

	for userID, score := range scoreTable {
		if skip(userID) {
			continue
		}
		if score == highestScore {
			winnerIDs = append(winnerIDs, userID)
		} else if score > highestScore {
			winnerIDs = winnerIDs[:0]
			winnerIDs = append(winnerIDs, userID)
			highestScore = score
		}
	}

	if len(winnerIDs) == 0 {
		// the only way this can pass is if the table is empty because the match list that produced the table was empty
		// a tournament with no matches is impossible since it should have never passed the IN_PROGRESS status
		panic("expected scoring table to produce at least one winner, got none")
	}
	return winnerIDs
}

func findTournamentWinner(ruleset model.TournamentRuleset, allMatches []CompletedPrevMatch) (int64, TieBreakerKind) {
	switch ruleset {
	case model.TournamentKnockout:
		// winner of the tournament is the player left standing
		return getKnockoutWinnerID(allMatches[len(allMatches)-1])
	case model.TournamentRoundRobin:
		winCountTable := makeWinCountTable(allMatches)

		// standard: player with the most total wins will win the tournament
		totalWinCheckWinnerIDs := findScoringTableWinners(winCountTable, func(int64) bool { return false })
		if len(totalWinCheckWinnerIDs) == 1 {
			return totalWinCheckWinnerIDs[0], TiebreakerNone
		}

		sonnebornTable := makeSonnebornTable(allMatches)

		// tiebreaker: use the Sonneborn-Berger score for each tied winner, largest wins
		sonnebornWinnerIDs := findScoringTableWinners(sonnebornTable, func(userID int64) bool { return !slices.Contains(totalWinCheckWinnerIDs, userID) })

		return sonnebornWinnerIDs[0], TiebreakerBySonnebornBerger
	case model.TournamentSwiss:
		swissScoreTables := makeSwissTable(allMatches)

		// standard: player with the highest swiss score will win the tournament
		swissScoresWinnerIDs := findScoringTableWinners(swissScoreTables, func(int64) bool { return false })
		if len(swissScoresWinnerIDs) == 1 {
			return swissScoresWinnerIDs[0], TiebreakerNone
		}

		sonnebornTable := makeSonnebornTable(allMatches)

		// tiebreaker: use the Sonneborn-Berger score for each tied winner, largest wins
		sonnebornWinnerIDs := findScoringTableWinners(sonnebornTable, func(userID int64) bool { return !slices.Contains(swissScoresWinnerIDs, userID) })

		return sonnebornWinnerIDs[0], TiebreakerBySonnebornBerger
	default:
		panic(fmt.Sprintf("unknown tournament ruleset %s", ruleset))
	}
}

func makeWinCountTable(allMatches []CompletedPrevMatch) map[int64]int32 {
	winCountTable := make(map[int64]int32)
	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			winCountTable[match.WhiteID] = winCountTable[match.WhiteID] + 1
		case model.BlackWin:
			winCountTable[match.BlackID] = winCountTable[match.BlackID] + 1
		default:
		}
	}
	return winCountTable
}

func makeSonnebornTable(allMatches []CompletedPrevMatch) map[int64]float64 {
	sonnebornTable := make(map[int64]float64)
	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo
		case model.BlackWin:
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo
		case model.Draw:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo/2
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo/2
		}
	}
	return sonnebornTable
}

func makeSwissTable(allMatches []CompletedPrevMatch) map[int64]float64 {
	swissScores := make(map[int64]float64)

	for _, match := range allMatches {
		swissScores[match.WhiteID] = 0.0
		swissScores[match.BlackID] = 0.0
	}

	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 1
		case model.BlackWin:
			swissScores[match.BlackID] = swissScores[match.BlackID] + 1
		case model.Draw:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 0.5
			swissScores[match.BlackID] = swissScores[match.BlackID] + 0.5
		}
	}
	return swissScores
}
