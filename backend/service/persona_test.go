package service

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupPersonaServices(t slogutil.TestLogger) (*PersonaService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewPersonaService(
		NewUserService(infra.Database),
		NewLeaderboardService(infra.Redis, infra.Querier()),
		NewSearchService(infra.Database),
	)

	return services, infra
}

func TestGetPersona(t *testing.T) {
	service, testinfra := setupPersonaServices(t)
	defer testinfra.Close()

	persona, err := service.GetPersona(t.Context(), 1, 10)
	require.NoError(t, err)

	wantPersona := model.Persona{
		User:  itest.TestUser[0],
		Stats: itest.TestUserStats[0],
		ReplayList: []model.FullReplay{
			itest.TestReplays[4],
			itest.TestReplays[3],
			itest.TestReplays[2],
			itest.TestReplays[0],
		},
	}

	testutil.Equal(t, persona, wantPersona)
}
