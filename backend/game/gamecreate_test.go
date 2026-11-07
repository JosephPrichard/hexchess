package game

import (
	"hexchess-svc/gamedata"
	"hexchess-svc/itest"
	"hexchess-svc/utils/slogutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupCreateTest(t slogutil.TestLogger) (*GameCreateService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewGameCreateService(
		infra.Redis,
		NewChessRepoService(infra.Redis),
	)

	return service, infra
}

func TestCreateGame(t *testing.T) {
	service, testinfra := setupCreateTest(t)
	defer testinfra.Close()

	gameID, err := service.CreateGame(t.Context(), ChessStateArgs{FirstColor: gamedata.White, Mode: gamedata.ModeCorrespondence1})
	require.NoError(t, err)

	wantGame := NewChessState(ChessStateArgs{
		ID:         gameID,
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.White,
	})

	wantGame.Game.InitPieceMoves()
	wantGame.Game.ClearTables()

	assertRedisChess(t, testinfra.Redis, wantGame, cmpOptsChessState)
}
