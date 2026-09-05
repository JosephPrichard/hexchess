package gameplay

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/utils/slogutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupCreateTest(t slogutil.TestLogger) (*GameCreateService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewGameCreateService(
		infra.Redis,
		gamestate.NewChessRepoService(infra.Redis),
	)

	return services, infra
}

func TestCreateGame(t *testing.T) {
	services, testinfra := setupCreateTest(t)
	defer testinfra.Close()

	gameID, err := services.CreateGame(t.Context(), model.White, model.ModeCorrespondence1, nil)
	require.NoError(t, err)

	wantGame := model.NewChessState(model.StateSetup{
		ID:         gameID,
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.White,
	})

	wantGame.Game.InitPieceMoves()
	wantGame.Game.ClearTables()

	assertRedisChess(t, testinfra.Redis, wantGame, cmpOptsChessState)
}
