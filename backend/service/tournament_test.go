package svc

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	"hexchess-svc/pb"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexchess-svc/lib/testutil"
	"testing"
	"time"
)

func TestCreateTournament(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{Entropy: &StableEntropySource{CurrTime: itest.TimeNow}}, itest.RWPostgres)
	defer services.Close()

	ctx := t.Context()

	key := uuid.New()

	tournamentID, err := services.CreateTournament(ctx, TournamentInst{
		Key:       key,
		Name:      "Tournaments 1",
		Rounds:    2,
		Mode:      model.ModeCorrespondence1,
		Countdown: time.Hour,
		CreatedOn: itest.TimeNow,
		CreatedBy: 1,
	})
	require.NoError(t, err)

	tournament, err := services.querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: key, Valid: true})
	require.NoError(t, err)

	wantTournament := sqlc.SelectTournamentByIDRow{
		ID:            tournamentID,
		Name:          "Tournaments 1",
		TournamentKey: pgtype.UUID{Bytes: key, Valid: true},
		Rounds:        2,
		Status:        sqlc.TournamentStatusEnum(model.TournamentLobby.String()),
		Ruleset:       sqlc.TournamentRulesetEnum(model.TournamentKnockout.String()),
		Countdown:     time.Hour.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		UpdatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		CreatedBy:     1,
		Mode:          sqlc.ModeEnum(model.ModeCorrespondence1.String()),
	}
	testutil.Equal(t, wantTournament, tournament)
}

func TestBeginTournamentCountdown(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	tests := []struct {
		name                      string
		tournamentKey             uuid.UUID
		userID                    int64
		wantBeginTourneyCountdown BeginTourneyCountdown
		wantErr                   error
		wantTournamenStatus       sqlc.SelectTournamentStatusRow
	}{
		{
			name:          "StatusPreconditionFailed_Scheduled",
			tournamentKey: itest.Tournament2ScheduledKnockoutKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "StatusPreconditionFailed_InProgress",
			tournamentKey: itest.Tournament5InProgressKnockoutKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "StatusPreconditionFailed_Finished",
			tournamentKey: itest.Tournament8FinishedKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "CountdownPermissions",
			tournamentKey: itest.Tournament0LobbyKey,
			userID:        2,
			wantErr:       ErrTournamentCountdownPermissions,
		},
		{
			name:                      "BeginTournamentCountdown",
			tournamentKey:             itest.Tournament0LobbyKey,
			userID:                    1,
			wantBeginTourneyCountdown: BeginTourneyCountdown{TournamentKey: itest.Tournament0LobbyKey},
			wantTournamenStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumSCHEDULED,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			result, err := services.BeginTournamentCountdown(ctx, tt.tournamentKey, tt.userID)

			assert.Equal(t, tt.wantBeginTourneyCountdown, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				status, err := services.querier.SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantTournamenStatus, status)

				outboxEvents, err := services.querier.SelectALLOutboxQueue(ctx)
				require.NoError(t, err)

				assert.Len(t, outboxEvents, 1)
			}
		})
	}
}

var sqlcTournamentParticipantCmpOpts = cmpopts.IgnoreFields(sqlc.TournamentParticipant{}, "JoinedOn")

func TestJoinTournament(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	tests := []struct {
		name             string
		inst             JoinTournamentInst
		wantResult       JoinTournamentEvent
		wantErr          error
		wantParticipants []sqlc.TournamentParticipant
	}{
		{
			name: "StatusPreconditionFailed",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament3ScheduledRoundRobinKey,
				JoiningUserID: 1,
				InsertionTime: time.Now(),
			},
			wantErr: ErrTournamentNotLobby,
		},
		{
			name: "ReachedMaxParticipants",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament1LobbyFilledKey,
				JoiningUserID: 1,
				InsertionTime: time.Now(),
			},
			wantErr: ErrTooManyParticipants,
		},
		{
			name: "JoinedTournament",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament0LobbyKey,
				JoiningUserID: 1,
				InsertionTime: time.Now(),
			},
			wantResult: JoinTournamentEvent{TournamentKey: itest.Tournament0LobbyKey, Mode: model.ModeCorrespondence1},
			wantParticipants: []sqlc.TournamentParticipant{
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament0LobbyKey, Valid: true},
					UserID:        1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			result, err := services.JoinTournament(ctx, tt.inst)

			assert.Equal(t, tt.wantResult, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				participants, err := services.querier.SelectParticipants(ctx, pgtype.UUID{Bytes: tt.inst.TournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantParticipants, participants, sqlcTournamentParticipantCmpOpts)
			}
		})
	}
}

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(sqlc.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestAdvanceTournament_StoresMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tournamentKey uuid.UUID
		eventID       uuid.UUID
		wantErr       error
		wantStatus    sqlc.SelectTournamentStatusRow
		wantMatches   []sqlc.TournamentMatch
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			wantStatus: sqlc.SelectTournamentStatusRow{
				Status: sqlc.TournamentStatusEnumINPROGRESS,
			},
			wantMatches: []sqlc.TournamentMatch{
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
			services, testinfra := setupServicesTest(t, serviceMocks{}, itest.RWPostgres, itest.Redis)
			defer services.Close()

			ctx := t.Context()

			eventID := tt.eventID
			if eventID == uuid.Nil {
				eventID = uuid.New()
			}
			_, err := services.advanceTournament(ctx, tt.tournamentKey, eventID)

			assert.Equal(t, tt.wantErr, errutil.LeafError(err))

			if tt.wantErr == nil {
				matches, err := testinfra.Querier.SelectMatches(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)
				testutil.Equal(t, tt.wantMatches, matches, sqlcTournamentMatchCmpOpts)

				status, err := testinfra.Querier.SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)
				testutil.Equal(t, tt.wantStatus, status)
			}
		})
	}
}

func TestAdvanceTournament_ThenGetChessStates(t *testing.T) {
	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres, itest.Redis)
	defer services.Close()

	ctx := t.Context()

	// since the eventID is stored in the idempotency keys table, we expect it to short circuit
	gameIDs, err := services.AdvanceTournament(ctx, uuid.New(), itest.TestEventID_TournamentCreation)
	require.NoError(t, err)

	wantGameIDs := []string{itest.TestEventID_TournamentCreation_GameID}
	assert.Equal(t, wantGameIDs, gameIDs)

	wantGames := map[string]*model.ChessState{
		itest.TestEventID_TournamentCreation_GameID: {
			ID:          itest.TestEventID_TournamentCreation_GameID,
			FirstColor:  model.White,
			WhitePlayer: model.PlayerState{ID: 1, Name: "user1", Country: "us", Present: true},
			BlackPlayer: model.PlayerState{ID: 2, Name: "user2", Country: "us", Present: true},
		},
	}

	for _, id := range gameIDs {
		chessState, err := services.GetChessState(ctx, id)
		require.NoError(t, err)

		testutil.Equal(t, wantGames[id], chessState, cmpopts.IgnoreFields(model.ChessState{}, "Game", "InitialBoard"))
	}
}

func TestAdvanceTournament_InsertsEvent(t *testing.T) {
	services, testinfra := setupServicesTest(t, serviceMocks{}, itest.RWPostgres, itest.Redis)
	defer services.Close()

	ctx := t.Context()

	tournamentKey := itest.Tournament2ScheduledKnockoutKey
	eventID := uuid.New()

	_, err := services.advanceTournament(ctx, tournamentKey, eventID)
	require.NoError(t, err)

	eventData, err := testinfra.Querier.SelectByEventID(ctx, pgtype.UUID{Bytes: eventID, Valid: true})
	require.NoError(t, err)

	var pbMatchCreations pb.MatchCreations
	require.NoError(t, proto.Unmarshal(eventData, &pbMatchCreations))

	wantTournaments := []*pb.MatchCreation{
		{
			Mode:    "CORRESPONDENCE_1",
			WhiteId: 3,
			BlackId: 4,
		},
		{
			Mode:    "CORRESPONDENCE_1",
			WhiteId: 5,
			BlackId: 6,
		},
	}
	testutil.Equal(t, wantTournaments, pbMatchCreations.Creations, protocmp.Transform(), protocmp.IgnoreFields(&pb.MatchCreation{}, "game_id"))
}
