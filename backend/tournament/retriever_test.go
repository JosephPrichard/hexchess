package tournament

import (
	"hexchess-svc/itest"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRetrieveTournamentTest(t slogutil.TestLogger) (*TournamentRetriever, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewRetrieveTournamentService(
		infra.Database.Querier,
		infra.Redis,
	)

	return service, infra
}

func TestGetTournament(t *testing.T) {
	tests := []struct {
		name          string
		tournamentKey uuid.UUID
		wantResp      FullTournament
		wantError     error
	}{
		{
			name:          "GotTournament",
			tournamentKey: itest.Tournament0LobbyKey,
			wantResp: FullTournament{
				Tournament:   TestTournaments[0],
				Participants: []Participant{},
				Matches:      []FullMatch{},
			},
		},
		{
			name:          "TournamentNotFound",
			tournamentKey: uuid.NewV7(),
			wantError:     ErrTournamentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, testinfra := setupRetrieveTournamentTest(t)
			defer testinfra.Close()

			ctx := t.Context()

			resp, err := service.GetTournament(ctx, tt.tournamentKey)
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
		wantResp  []Tournament
		wantError error
	}{
		{
			name: "RetrievedTournaments",
			wantResp: []Tournament{
				TestTournaments[9],
				TestTournaments[8],
				TestTournaments[7],
				TestTournaments[6],
				TestTournaments[5],
				TestTournaments[4],
				TestTournaments[3],
				TestTournaments[2],
				TestTournaments[1],
				TestTournaments[0],
			},
		},
		{
			name:   "RetrievedTournamentsForParticipant",
			userID: opt.Some(int64(4)),
			wantResp: []Tournament{
				TestTournaments[5],
				TestTournaments[2],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, testinfra := setupRetrieveTournamentTest(t)
			defer testinfra.Close()

			tournaments, err := service.GetTournaments(t.Context(), tt.userID, tt.afterID, 10)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantResp, tournaments)
		})
	}
}
