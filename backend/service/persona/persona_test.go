package persona

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/service/replay"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupPersonaServices(t alog.TestLogger, flags ...itest.TestFlag) (*PersonaService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewPersonaService(
		user.NewUserService(infra.Database),
		leaderboard.NewLeaderboardService(infra.Redis, infra.Querier()),
		replay.NewSearchService(infra.Database),
	)

	return services, infra
}

func TestGetPersona(t *testing.T) {
	t.Parallel()

	service, testinfra := setupPersonaServices(t, itest.ROPostgres, itest.Redis)
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
