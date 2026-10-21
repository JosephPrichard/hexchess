package service

import (
	"hexchess-svc/database/testquery"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/slogutil"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexchess-svc/utils/testutil"
	"testing"
	"time"
)

func setupUpdateTournamentTest(t slogutil.TestLogger) (*TournamentUpdater, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewTournamentUpdater(
		infra.Database,
		infra.Redis,
		producers.NewRiverProducer(&producers.NoopRiverClient{}),
	)

	return service, infra
}

func TestCreateTournament(t *testing.T) {
	service, testinfra := setupUpdateTournamentTest(t)
	defer testinfra.Close()
	ctx := t.Context()

	key := uuid.New()

	tournamentID, err := service.CreateTournament(ctx, TournamentInst{
		Key:       key,
		Name:      "Tournaments 1",
		Rounds:    2,
		Mode:      model.ModeCorrespondence1,
		Countdown: time.Hour,
		CreatedOn: itest.TimeNow,
		CreatedBy: 1,
	})
	require.NoError(t, err)

	tournament, err := testinfra.Querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: key, Valid: true})
	require.NoError(t, err)

	wantTournament := testquery.Tournament{
		ID:            tournamentID,
		Name:          "Tournaments 1",
		TournamentKey: pgtype.UUID{Bytes: key, Valid: true},
		Rounds:        2,
		Status:        "LOBBY",
		Ruleset:       "KNOCKOUT",
		Countdown:     time.Hour.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		UpdatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		CreatedBy:     1,
		Mode:          "CORRESPONDENCE_1",
	}
	testutil.Equal(t, wantTournament, tournament)
}

func TestBeginTournamentCountdown(t *testing.T) {
	service, testinfra := setupUpdateTournamentTest(t)
	defer testinfra.Close()

	tests := []struct {
		name                      string
		tournamentKey             uuid.UUID
		userID                    int64
		wantBeginTourneyCountdown BeginTourneyCountdown
		wantErr                   error
		wantTournamentStatus      testquery.TournamentStatusEnum
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
			wantTournamentStatus:      testquery.TournamentStatusEnumSCHEDULED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			result, err := service.BeginTournamentCountdown(ctx, tt.tournamentKey, tt.userID)

			assert.Equal(t, tt.wantBeginTourneyCountdown, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				tournament, err := testinfra.Querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantTournamentStatus, tournament.Status)
			}
		})
	}
}

var sqlcTournamentParticipantCmpOpts = cmpopts.IgnoreFields(testquery.TournamentParticipant{}, "JoinedOn")

func TestJoinTournament(t *testing.T) {
	service, testinfra := setupUpdateTournamentTest(t)
	defer testinfra.Close()

	tests := []struct {
		name             string
		inst             JoinTournamentInst
		wantResult       JoinTournamentEvent
		wantErr          error
		wantParticipants []testquery.TournamentParticipant
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
			wantParticipants: []testquery.TournamentParticipant{
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

			result, err := service.JoinTournament(ctx, tt.inst)

			assert.Equal(t, tt.wantResult, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				participants, err := testinfra.Querier.SelectParticipants(ctx, pgtype.UUID{Bytes: tt.inst.TournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantParticipants, participants, sqlcTournamentParticipantCmpOpts)
			}
		})
	}
}
