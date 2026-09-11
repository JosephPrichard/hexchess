package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/database"
	"hexchess-svc/database/mutator"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type TournamentAdvanceService struct {
	// infra dependencies
	database.Database
	broadcaster pubsub.Broadcaster

	// service dependencies
	user     UserGetter
	gameplay GameCreator
}

type UserGetter interface {
	SelectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error)
}

type GameCreator interface {
	SetupGame(ctx context.Context, setup model.StateSetup) error
}

func NewTournamentAdvanceService(
	database database.Database,
	user UserGetter,
	gameplay GameCreator,
	broadcaster pubsub.Broadcaster,
) *TournamentAdvanceService {
	return &TournamentAdvanceService{Database: database, user: user, gameplay: gameplay, broadcaster: broadcaster}
}

func (services *TournamentAdvanceService) AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.GameID, error) {
	defer perf.WithContext(ctx).Log()

	// progress tournament on the database
	matches, err := services.ProgressTournament(ctx, tournamentKey, eventID)
	if err != nil {
		return nil, serrors.New("advance tournament", err, "tournamentKey", tournamentKey)
	}

	// create playable games correlated with the committed matches
	userDataMap, err := services.constructMatchDataMap(ctx, matches)
	if err != nil {
		return nil, err
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for i, match := range matches {
		eg.Go(func() error {
			return services.setupGame(egCtx, i, match, userDataMap)
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// notify tournament participants that tournament has been progressed with new matches
	// TODO: we need to select FullMatch information and broadcast that
	services.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:  tournamentKey.String(),
		Kind: model.TournamentMatchmakingKind,
		// Matches: matches,
	})

	var matchGameIDs []model.GameID
	for _, match := range matches {
		matchGameIDs = append(matchGameIDs, match.GameID)
	}
	return matchGameIDs, nil
}

var ExpectedAdvanceTournamentStatus = []model.TournamentStatus{model.TournamentScheduled, model.TournamentInProgress}

func (services *TournamentAdvanceService) ProgressTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.MatchCreation, error) {
	defer perf.WithContext(ctx).Log()

	var matchesToCreate []model.MatchCreation

	err := services.Database.ExecTx(ctx, database.TxArgs{
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
		QueryFn: func(ctx context.Context, _ pgx.Tx, query database.QuerierMutator) error {
			return progressTournament(ctx, query, tournamentKey, eventID, &matchesToCreate)
		},
	})

	return matchesToCreate, err
}

func progressTournament(ctx context.Context, query database.QuerierMutator, tournamentKey uuid.UUID, eventID uuid.UUID, matchesToCreate *[]model.MatchCreation) error {
	previousEvent, err := selectPreviousAdvanceEvent(ctx, query, eventID)
	if err != nil {
		return serrors.New("select previous advance tournament event", err, "eventID", eventID)
	}
	if previousEvent != nil {
		*matchesToCreate = previousEvent.Creations
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

	*matchesToCreate = matchmaking.NextMatches
	slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey, "matchesToCreate", matchesToCreate)
	return nil
}

func selectPreviousAdvanceEvent(ctx context.Context, query database.QuerierMutator, eventID uuid.UUID) (*model.MatchCreations, error) {
	var matchCreations model.MatchCreations

	eventOutput, err := query.SelectByEventKeyID(ctx, pgtype.UUID{Bytes: eventID, Valid: true})
	switch {
	case database.IsErrNoRows(err):
		slog.InfoContext(ctx, "advance tournament: event id not consumed", "eventID", eventID)
		return nil, nil
	case err != nil:
		return nil, err
	default:
		err := sonic.Unmarshal(eventOutput, &matchCreations)
		return &matchCreations, err
	}
}

func advanceScheduledTournament(ctx context.Context, query database.QuerierMutator, tournament query.SelectTournamentByIDRow) (MatchmakingOutput, error) {
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

func advanceInProgressTournament(ctx context.Context, query database.QuerierMutator, tournament query.SelectTournamentByIDRow) (MatchmakingOutput, error) {
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
	query database.QuerierMutator,
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

func insertTournamentMatches(ctx context.Context, query database.QuerierMutator, tournamentKey pgtype.UUID, response MatchmakingOutput) error {
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

func (services *TournamentAdvanceService) constructMatchDataMap(ctx context.Context, matches []model.MatchCreation) (map[int64]model.User, error) {
	userIDs := make([]int64, 0, len(matches)*2)
	for _, match := range matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	users, err := services.user.SelectUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, serrors.New("select user player data by ids", err)
	}
	userDataMap := make(map[int64]model.User)
	for _, userAccount := range users {
		userDataMap[userAccount.ID] = userAccount
	}

	return userDataMap, nil
}

func (services *TournamentAdvanceService) setupGame(
	ctx context.Context, index int, match model.MatchCreation, userDataMap map[int64]model.User,
) error {
	whitePlayerData, okWhite := userDataMap[match.WhiteID]
	blackPlayerData, okBlack := userDataMap[match.BlackID]

	if !okWhite || !okBlack {
		// invariant: white and black should be valid ids if they have been pushed to the queue
		return fmt.Errorf("missing player data for match: %+v", match)
	}

	err := services.gameplay.SetupGame(ctx, model.StateSetup{
		ID:         match.GameID,
		Mode:       match.GameMode,
		FirstColor: model.White,
		White:      model.NewPlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
		Black:      model.NewPlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
	})
	if err != nil {
		return serrors.New("create game", err, "index", index, "gameID", match.GameID)
	}

	return nil
}

type TournamentStatusAssertionError struct {
	Expected []model.TournamentStatus
	Got      model.TournamentStatus
}

func (e TournamentStatusAssertionError) Error() string {
	return fmt.Sprintf("tournament state is invalid: expected %v, got %v", e.Expected, e.Got)
}
