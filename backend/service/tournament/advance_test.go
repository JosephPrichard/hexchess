package tournament

import (
	"errors"
	"hexchess-svc/database/query"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/gamestate"
	userSvc "hexchess-svc/service/user"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdvanceTest(t alog.TestLogger) (*TournamentAdvanceService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewTournamentAdvanceService(
		infra.Database,
		userSvc.NewUserService(infra.Database),
		gameplay.NewGameCreateService(infra.Redis, gamestate.NewChessRepoService(infra.Redis)),
		pubsub.NewSyncBroadcaster(infra.Redis),
	)

	return services, infra
}

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(query.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestProgressTournament_StoresMatches(t *testing.T) {
	tests := []struct {
		name          string
		tournamentKey uuid.UUID
		eventID       uuid.UUID
		wantErr       error
		wantStatus    query.SelectTournamentStatusRow
		wantMatches   []query.TournamentMatch
	}{
		// Precondition
		{
			name:          "StatusPreconditionFailed_Lobby",
			tournamentKey: itest.Tournament1LobbyFilledKey,
			wantErr: MatchInvariantError{
				TournamentKey: itest.Tournament1LobbyFilledKey,
				Err: TournamentStatusAssertionError{
					Expected: []model.TournamentStatus{model.TournamentScheduled, model.TournamentInProgress},
					Got:      model.TournamentLobby,
				},
			},
		},
		{
			name:          "StatusPreconditionFailed_Finished",
			tournamentKey: itest.Tournament8FinishedKey,
			wantErr: MatchInvariantError{
				TournamentKey: itest.Tournament8FinishedKey,
				Err: TournamentStatusAssertionError{
					Expected: []model.TournamentStatus{model.TournamentScheduled, model.TournamentInProgress},
					Got:      model.TournamentFinished,
				},
			},
		},
		// Start
		{
			name:          "StartingKnockoutTournament",
			tournamentKey: itest.Tournament2ScheduledKnockoutKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// tournament has 2 rounds with join order of [1,2,3,4], so starting the tournament creates 2 rounds wso the matches go 1-2, 3-4
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
					Round:         1,
					WhiteID:       3,
					BlackID:       4,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
					Round:         1,
					WhiteID:       5,
					BlackID:       6,
				},
			},
		},
		{
			name:          "StartingRoundRobinTournament",
			tournamentKey: itest.Tournament3ScheduledRoundRobinKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// tournament has 4 participants with join order of [1,2,3,4], so the matches go 1-2, 3-4
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament3ScheduledRoundRobinKey, Valid: true},
					Round:         1,
					WhiteID:       1,
					BlackID:       2,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament3ScheduledRoundRobinKey, Valid: true},
					Round:         1,
					WhiteID:       5,
					BlackID:       6,
				},
			},
		},
		{
			name:          "StartingSwissTournament",
			tournamentKey: itest.Tournament4ScheduledSwissKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// tournament has 4 participants with elo ordering of [6,5,2,1] for Correspondence1, so the matches go 6-2, 5-1
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament4ScheduledSwissKey, Valid: true},
					Round:         1,
					WhiteID:       6,
					BlackID:       2,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament4ScheduledSwissKey, Valid: true},
					Round:         1,
					WhiteID:       5,
					BlackID:       1,
				},
			},
		},
		// Advance
		{
			name:          "AdvanceKnockoutTournament",
			tournamentKey: itest.Tournament5InProgressKnockoutKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// should not delete previous matches
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament5InProgressKnockoutKey, Valid: true},
					Round:         1,
					WhiteID:       10,
					BlackID:       11,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament5InProgressKnockoutKey, Valid: true},
					Round:         1,
					WhiteID:       12,
					BlackID:       13,
				},
				// tournament has 2 matches, so we expect 1 more match
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament5InProgressKnockoutKey, Valid: true},
					Round:         2,
					WhiteID:       10,
					BlackID:       12,
				},
			},
		},
		{
			name:          "AdvanceRoundRobinTournament",
			tournamentKey: itest.Tournament6InProgressRoundRobinKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// should not delete previous matches
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament6InProgressRoundRobinKey, Valid: true},
					Round:         1,
					WhiteID:       10,
					BlackID:       11,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament6InProgressRoundRobinKey, Valid: true},
					Round:         1,
					WhiteID:       12,
					BlackID:       13,
				},
				// tournament has 2 matches, so we expect 2 more matches
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament6InProgressRoundRobinKey, Valid: true},
					Round:         2,
					WhiteID:       10,
					BlackID:       13,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament6InProgressRoundRobinKey, Valid: true},
					Round:         2,
					WhiteID:       11,
					BlackID:       12,
				},
			},
		},
		{
			name:          "AdvanceSwissTournament",
			tournamentKey: itest.Tournament7InProgressSwissKey,
			wantStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []query.TournamentMatch{
				// should not delete previous matches
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament7InProgressSwissKey, Valid: true},
					Round:         1,
					WhiteID:       10,
					BlackID:       11,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament7InProgressSwissKey, Valid: true},
					Round:         1,
					WhiteID:       12,
					BlackID:       13,
				},
				// tournament has 2 matches, so we expect 2 more matches
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament7InProgressSwissKey, Valid: true},
					Round:         2,
					WhiteID:       10,
					BlackID:       12,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament7InProgressSwissKey, Valid: true},
					Round:         2,
					WhiteID:       11,
					BlackID:       13,
				},
			},
		},
		{
			name:          "AdvanceUncompletedTournamentMatches",
			tournamentKey: itest.Tournament9InProgressUncompletedKey,
			wantErr:       MatchInvariantError{TournamentKey: itest.Tournament9InProgressUncompletedKey, Err: ErrMatchRoundCount},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupAdvanceTest(t)
			defer testinfra.Close()

			ctx := t.Context()

			eventID := tt.eventID
			if eventID == uuid.Nil {
				eventID = uuid.New()
			}
			_, err := services.AdvanceTournament(ctx, tt.tournamentKey, eventID)

			assert.Equal(t, tt.wantErr, leafError(err))

			if tt.wantErr == nil {
				matches, err := testinfra.Querier().SelectMatches(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)
				testutil.Equal(t, tt.wantMatches, matches, sqlcTournamentMatchCmpOpts)

				status, err := testinfra.Querier().SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)
				testutil.Equal(t, tt.wantStatus, status)
			}
		})
	}
}

func leafError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

var cmpOptsChessState = cmpopts.IgnoreFields(model.ChessState{}, "Game", "InitialBoard", "StartTime")

func TestProgressTournament_ThenGetChessStates(t *testing.T) {
	services, testinfra := setupAdvanceTest(t)
	defer testinfra.Close()
	ctx := t.Context()

	// since the eventID is stored in the idempotency keys table, we expect it to short circuit
	gameIDs, err := services.AdvanceTournament(ctx, uuid.New(), itest.TestEventID_TournamentCreation)
	require.NoError(t, err)

	wantGameIDs := []model.GameID{itest.TestEventID_TournamentCreation_GameID}
	assert.Equal(t, wantGameIDs, gameIDs)

	wantGames := map[model.GameID]*model.ChessState{
		itest.TestEventID_TournamentCreation_GameID: {
			ID:          itest.TestEventID_TournamentCreation_GameID,
			FirstColor:  model.White,
			WhitePlayer: model.PlayerState{ID: 1, Name: "user1", Country: "us", Present: true},
			BlackPlayer: model.PlayerState{ID: 2, Name: "user2", Country: "us", Present: true},
		},
	}

	for _, id := range gameIDs {
		chessState, err := gamestate.NewChessRepoService(testinfra.Redis).GetChessState(ctx, id)
		require.NoError(t, err)

		testutil.Equal(t, wantGames[id], chessState, cmpOptsChessState)
	}
}

var cmpOptsMatchCreation = cmpopts.IgnoreFields(model.MatchCreation{}, "GameID")

func TestProgressTournament_InsertsEvent(t *testing.T) {
	services, testinfra := setupAdvanceTest(t)
	defer testinfra.Close()
	ctx := t.Context()

	tournamentKey := itest.Tournament2ScheduledKnockoutKey
	eventID := uuid.New()

	_, err := services.AdvanceTournament(ctx, tournamentKey, eventID)
	require.NoError(t, err)

	eventData, err := testinfra.Querier().SelectByEventKeyID(ctx, pgtype.UUID{Bytes: eventID, Valid: true})
	require.NoError(t, err)

	var matchCreations model.MatchCreations
	require.NoError(t, sonic.Unmarshal(eventData, &matchCreations))

	wantTournaments := []model.MatchCreation{
		{
			GameMode: model.ModeCorrespondence1,
			WhiteID:  3,
			BlackID:  4,
		},
		{
			GameMode: model.ModeCorrespondence1,
			WhiteID:  5,
			BlackID:  6,
		},
	}
	testutil.Equal(t, wantTournaments, matchCreations.Creations, cmpOptsMatchCreation)
}
