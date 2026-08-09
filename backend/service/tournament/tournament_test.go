package tournament

import (
	"hexchess-svc/database/query"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/alog"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexchess-svc/utils/testutil"
	"testing"
	"time"
)

func setupTest(t alog.TestLogger, flags ...itest.TestFlag) (*TournamentService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewTournamentService(
		infra.Database,
		infra.Redis,
		producers.NewRiverProducer(&producers.NoopRiverClient{}),
	)

	return services, infra
}

func TestCreateTournament(t *testing.T) {
	services, testinfra := setupTest(t, itest.RWPostgres)
	defer testinfra.Close()

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

	tournament, err := testinfra.Querier().SelectTournamentByID(ctx, pgtype.UUID{Bytes: key, Valid: true})
	require.NoError(t, err)

	wantTournament := query.SelectTournamentByIDRow{
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
	services, testinfra := setupTest(t, itest.RWPostgres)
	defer testinfra.Close()

	tests := []struct {
		name                      string
		tournamentKey             uuid.UUID
		userID                    int64
		wantBeginTourneyCountdown BeginTourneyCountdown
		wantErr                   error
		wantTournamentStatus      query.SelectTournamentStatusRow
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
			wantTournamentStatus: query.SelectTournamentStatusRow{
				Status: query.TournamentStatusEnumSCHEDULED,
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
				status, err := testinfra.Querier().SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantTournamentStatus, status)
			}
		})
	}
}

var sqlcTournamentParticipantCmpOpts = cmpopts.IgnoreFields(query.TournamentParticipant{}, "JoinedOn")

func TestJoinTournament(t *testing.T) {
	services, testinfra := setupTest(t, itest.RWPostgres)
	defer testinfra.Close()

	tests := []struct {
		name             string
		inst             JoinTournamentInst
		wantResult       JoinTournamentEvent
		wantErr          error
		wantParticipants []query.TournamentParticipant
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
			wantParticipants: []query.TournamentParticipant{
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
				participants, err := testinfra.Querier().SelectParticipants(ctx, pgtype.UUID{Bytes: tt.inst.TournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantParticipants, participants, sqlcTournamentParticipantCmpOpts)
			}
		})
	}
}
