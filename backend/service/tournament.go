package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func (services *HexchessServices) GetTournament(ctx context.Context, tournamentKey uuid.UUID) (t model.FullTournament, err error) {
	var tournamentRow sqlc.SelectTournamentByIDRow
	var matchRows []sqlc.SelectReplayMatchesByTournamentIDRow
	var participantRows []sqlc.SelectParticipantsWithUserByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = services.querier.SelectTournamentByID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select tournament by key %v", tournamentKey)
	})

	eg.Go(func() (err error) {
		participantRows, err = services.querier.SelectParticipantsWithUserByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
		return errutil.Guardf(err, "select participants by tournament key %v", tournamentKey)
	})

	eg.Go(func() (err error) {
		matchRows, err = services.querier.SelectReplayMatchesByTournamentID(egCtx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
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

	userLdbRanksMap, err := services.getUsersLeaderboardRank(ctx, participantIDs, tournament.Mode)
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
			Replay:          replay,
			ReplayColorElos: model.MakeReplayView(replay),
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

func (services *HexchessServices) GetTournaments(ctx context.Context, participantID enum.Optional[int64], afterID enum.Optional[int64], perPage int32) ([]model.Tournament, error) {
	if !afterID.IsPresent {
		afterID.Value = int64(math.MaxInt64)
	}

	var tournaments []model.Tournament

	if participantID.IsPresent {
		tournamentRows, err := services.querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID.Value,
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments by participant %v after id %v: %w", participantID, afterID, err)
		}
		tournaments = mapTournamentRows(tournamentRows, func(t sqlc.SelectTournamentsByParticipantRow) model.Tournament {
			return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
		})
	} else {
		tournamentRows, err := services.querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID.Value,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("select tournaments after id %v: %w", afterID, err)
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

func (services *HexchessServices) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
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

	tournamentID, err := services.querier.InsertTournament(ctx, sqlc.InsertTournamentParams{
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

func (services *HexchessServices) LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error) {
	deletedIDs, err := services.querier.DeleteTournamentParticipant(ctx, sqlc.DeleteTournamentParticipantParams{
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

type JoinTournamentEvent struct {
	TournamentKey uuid.UUID
	Mode          model.GameMode
}

func (services *HexchessServices) JoinTournament(ctx context.Context, inst JoinTournamentInst) (JoinTournamentEvent, error) {
	var result JoinTournamentEvent

	err := services.transactor.ExecTx(ctx, db.TxArgs{
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

func (services *HexchessServices) BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error) {
	var tourneyCountdown BeginTourneyCountdown

	err := services.transactor.ExecTx(ctx, db.TxArgs{
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

			if err := producers.PublishScheduledTournamentEvent(ctx, querier, tournamentKey, scheduledOn); err != nil {
				return fmt.Errorf("publish scheduled tournament %s event: %w", tournamentKey, err)
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

func (services *HexchessServices) AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID) error {
	return services.transactor.ExecTx(ctx, db.TxArgs{
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
					return fmt.Errorf("matchmake scheduled tournament %s: %w", tournamentKey, err)
				}
			case model.TournamentInProgress:
				if response, err = matchmakeInProgressTournament(ctx, querier, tournamentRow); err != nil {
					return fmt.Errorf("matchmake in progress tournament %s: %w", tournamentKey, err)
				}
			default:
				return MatchInvariantError{
					TournamentKey: tournamentKey,
					Err:           TournamentStatusAssertionError{Got: status, Expected: ExpectedAdvanceTournamentStatus},
				}
			}

			if err := insertTournamentMatches(ctx, querier, tournamentKey, response); err != nil {
				return fmt.Errorf("insert tournament %s matches: %w", tournamentKey, err)
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

		if err := producers.PublishCreateTournamentMatchesEvent(ctx, querier, tournamentKey, response.NextMatches); err != nil {
			return fmt.Errorf("publish create tournament %s matches event: %w", tournamentKey, err)
		}
	}

	return nil
}

func (services *HexchessServices) BroadcastTournamentParticipant(ctx context.Context, playerID int64, tournamentJoin JoinTournamentEvent) error {
	lbdUser, err := services.GetLeaderboardUser(ctx, playerID, tournamentJoin.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user", "Err", err)
	}
	if err := services.broadcaster.BroadcastTournament(ctx, model.SerializeParticipantOutput(tournamentJoin.TournamentKey, lbdUser)); err != nil {
		return fmt.Errorf("broadcast tournament %+v participant: %w", tournamentJoin, err)
	}
	return err
}
