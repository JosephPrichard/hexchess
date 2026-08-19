package tournament

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRetrieveTest(t alog.TestLogger) (*RetrieveTournamentService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewRetrieveTournamentService(
		infra.Database.Querier(),
		infra.Redis,
	)

	return services, infra
}

func TestGetTournament(t *testing.T) {
	tests := []struct {
		name          string
		tournamentKey uuid.UUID
		wantResp      model.FullTournament
		wantError     error
	}{
		{
			name:          "GotTournament",
			tournamentKey: itest.Tournament0LobbyKey,
			wantResp: model.FullTournament{
				Tournament:   itest.Tournaments[0],
				Participants: []model.Participant{},
				Matches:      []model.FullMatch{},
			},
		},
		{
			name:          "TournamentNotFound",
			tournamentKey: uuid.New(),
			wantError:     ErrTournamentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupRetrieveTest(t)
			defer testinfra.Close()

			ctx := t.Context()

			resp, err := services.GetTournament(ctx, tt.tournamentKey)
			assert.Equal(t, tt.wantError, err)

			testutil.Equal(t, tt.wantResp, resp)
		})
	}
}

func TestGetTournaments(t *testing.T) {
	tests := []struct {
		name      string
		userID    opt.Option[int64]
		afterID   opt.Option[int64]
		wantResp  []model.Tournament
		wantError error
	}{
		{
			name: "RetrievedTournaments",
			wantResp: []model.Tournament{
				itest.Tournaments[9],
				itest.Tournaments[8],
				itest.Tournaments[7],
				itest.Tournaments[6],
				itest.Tournaments[5],
				itest.Tournaments[4],
				itest.Tournaments[3],
				itest.Tournaments[2],
				itest.Tournaments[1],
				itest.Tournaments[0],
			},
		},
		{
			name:   "RetrievedTournamentsForParticipant",
			userID: opt.Some(int64(4)),
			wantResp: []model.Tournament{
				itest.Tournaments[5],
				itest.Tournaments[2],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupRetrieveTest(t)
			defer testinfra.Close()

			tournaments, err := services.GetTournaments(t.Context(), tt.userID, tt.afterID, 10)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantResp, tournaments)
		})
	}
}
