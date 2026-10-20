package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/database"
	"hexchess-svc/database/mutate"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type TournamentAdvancer struct {
	// infra
	database.Database
	broadcaster pubsub.Broadcaster

	// service
	user     UserGetter
	gameplay GameCreator
}

type UserGetter interface {
	SelectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error)
}

type GameCreator interface {
	SetupGame(ctx context.Context, setup model.StateSetup) error
}

func NewTournamentAdvancer(
	database database.Database,
	user UserGetter,
	gameplay GameCreator,
	broadcaster pubsub.Broadcaster,
) *TournamentAdvancer {
	return &TournamentAdvancer{Database: database, user: user, gameplay: gameplay, broadcaster: broadcaster}
}

func (service *TournamentAdvancer) AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.GameID, error) {
	defer perf.WithContext(ctx).Log()

	// advance tournament on the database
	matches, err := service.advanceTournament(ctx, tournamentKey, eventID)
	if err != nil {
		return nil, errutil.New("advance tournament", err, "key", tournamentKey)
	}

	// create playable games correlated with the committed matches
	userDataMap, err := service.constructMatchDataMap(ctx, matches)
	if err != nil {
		return nil, err
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for i, match := range matches {
		eg.Go(func() error {
			return service.setupGame(egCtx, i, match, userDataMap)
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// notify tournament participants that tournament has been progressed with new matches
	service.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:     tournamentKey.String(),
		Kind:    model.TournamentMatchmakingKind,
		Matches: matches,
	})

	var matchGameIDs []model.GameID
	for _, match := range matches {
		matchGameIDs = append(matchGameIDs, match.GameID)
	}
	return matchGameIDs, nil
}

var ExpectedAdvanceTournamentStatus = []model.TournamentStatus{model.TournamentScheduled, model.TournamentInProgress}

func (service *TournamentAdvancer) advanceTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]model.FullMatch, error) {
	defer perf.WithContext(ctx).Log()

	var createdMatches []model.FullMatch

	err := service.Database.ExecTx(ctx, database.TxArgs{
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
			return advanceTournament(ctx, query, tournamentKey, eventID, &createdMatches)
		},
	})

	return createdMatches, err
}

func advanceTournament(ctx context.Context, query database.QuerierMutator, tournamentKey uuid.UUID, eventID uuid.UUID, createdMatches *[]model.FullMatch) error {
	previousEvent, err := selectPreviousAdvanceEvent(ctx, query, eventID)
	if err != nil {
		return errutil.New("select previous advance tournament event", err, "eventID", eventID)
	}
	if previousEvent != nil {
		*createdMatches = previousEvent.Creations
		return nil
	}

	tournament, err := query.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	if err != nil {
		return errutil.New("select tournament by key", err, "key", tournamentKey)
	}
	status := enum.Expect(tournament.Status, model.TournamentStatusEnums)

	var matchmaking MatchmakingOutput

	switch status {
	case model.TournamentScheduled:
		output, err := advanceScheduledTournament(ctx, query, tournament)
		if err != nil {
			return errutil.New("advance scheduled tournament", err)
		}
		matchmaking = output
	case model.TournamentInProgress:
		output, err := advanceInProgressTournament(ctx, query, tournament)
		if err != nil {
			return errutil.New("advance in progress tournament", err)
		}
		matchmaking = output
	default:
		return NewMatchInvariantError(tournamentKey, TournamentStatusAssertionError{Got: status, Expected: ExpectedAdvanceTournamentStatus})
	}

	insertedMatches, err := insertTournamentMatches(ctx, query, eventID, tournamentKey, matchmaking)
	if err != nil {
		return err
	}

	*createdMatches = insertedMatches
	slog.InfoContext(ctx, "advanced tournament", "key", tournamentKey, "createdMatches", createdMatches)
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
		return MatchmakingOutput{}, errutil.New("select participant ids by tournament key", err, "tournamentKey", tournament.TournamentKey)
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
		return MatchmakingOutput{}, errutil.New("select matches by tournament key", err, "tournamentKey", tournament.TournamentKey)
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

func insertTournamentMatches(
	ctx context.Context, query database.QuerierMutator, eventID uuid.UUID, tournamentKey uuid.UUID, output MatchmakingOutput,
) ([]model.FullMatch, error) {
	matchCreations := output.NextMatches

	// step one: batch insert matches into database (and retrieve the state)
	var matchInsts []mutate.BatchInsertTournamentMatchParams
	for _, match := range matchCreations {
		matchInsts = append(matchInsts, mutate.BatchInsertTournamentMatchParams{
			TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
			Round:         output.NextMatchRound,
			GameID:        match.GameID.String(),
			WhiteID:       match.WhiteID,
			BlackID:       match.BlackID,
		})
	}

	var rows []mutate.TournamentMatch
	var batchErr error

	query.BatchInsertTournamentMatch(ctx, matchInsts).QueryRow(func(i int, row mutate.TournamentMatch, err error) {
		rows = append(rows, row)
		batchErr = errors.Join(batchErr, err)
	})
	if batchErr != nil {
		return nil, batchErr
	}

	var matches []model.FullMatch
	for i := range rows {
		row := rows[i]
		matchCreation := matchCreations[i]

		matches = append(matches, model.FullMatch{
			Ordering:      row.Ordering,
			GameID:        model.GameID(row.GameID),
			TournamentKey: row.TournamentKey.Bytes,
			Round:         row.Round,
			CreatedOn:     row.CreatedOn.Time,
			WhiteID:       row.WhiteID,
			BlackID:       row.BlackID,
			GameMode:      opt.Some(matchCreation.GameMode),
			Replay:        nil, // guaranteed to always be nil, a recently inserted match cannot have a replay (idempotency key will no longer be used by the time it does)
		})
	}

	// step two: insert idempotency key (contains inserted matches)
	eventInput, err := sonic.Marshal(model.MatchCreations{Creations: matches})
	if err != nil {
		return nil, err
	}
	if err := query.InsertEventKey(ctx, mutate.InsertEventKeyParams{
		ID:   pgtype.UUID{Bytes: eventID, Valid: true},
		Data: eventInput,
	}); err != nil {
		return nil, errutil.New("insert event with data", err, "eventID", eventID, "matchCreations", matches)
	}

	// step 3: update tournament status
	shouldUpdateWinnerID := output.WinnerID != WinnerIDNone

	if err := query.UpdateTournamentStatus(ctx, mutate.UpdateTournamentStatusParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		Status:        mutate.TournamentStatusEnum(output.NextStatus.String()),
		Rounds:        pgtype.Int4{Int32: output.TotalRounds, Valid: true},
		WinnerID:      pgtype.Int8{Int64: output.WinnerID, Valid: shouldUpdateWinnerID},
	}); err != nil {
		return nil, errutil.New("update tournament status", err, "key", tournamentKey, "output", output)
	}

	return matches, nil
}

func (service *TournamentAdvancer) constructMatchDataMap(ctx context.Context, matches []model.FullMatch) (map[int64]model.User, error) {
	userIDs := make([]int64, 0, len(matches)*2)
	for _, match := range matches {
		userIDs = append(userIDs, match.WhiteID, match.BlackID)
	}
	users, err := service.user.SelectUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, errutil.New("select user player data by ids", err)
	}
	userDataMap := make(map[int64]model.User)
	for _, userAccount := range users {
		userDataMap[userAccount.ID] = userAccount
	}

	return userDataMap, nil
}

func (service *TournamentAdvancer) setupGame(ctx context.Context, index int, match model.FullMatch, userDataMap map[int64]model.User) error {
	whitePlayerData, okWhite := userDataMap[match.WhiteID]
	blackPlayerData, okBlack := userDataMap[match.BlackID]

	if !okWhite || !okBlack {
		// invariant: white and black should be valid ids if they have been pushed to the queue
		return fmt.Errorf("missing player data for match: %+v", match)
	}
	if !match.GameMode.Present {
		// invariant: game mode is provided for this full match
		return fmt.Errorf("missing game mode for match: %+v", match)
	}

	err := service.gameplay.SetupGame(ctx, model.StateSetup{
		ID:         match.GameID,
		Mode:       match.GameMode.Value,
		FirstColor: model.White,
		White:      model.NewPlayer(match.WhiteID, whitePlayerData.Username, whitePlayerData.Country),
		Black:      model.NewPlayer(match.BlackID, blackPlayerData.Username, blackPlayerData.Country),
	})
	if err != nil {
		return errutil.New("create game", err, "index", index, "gameID", match.GameID)
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
