package service

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
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

	gameID, err := service.CreateGame(t.Context(), model.ChessStateArgs{FirstColor: model.White, Mode: model.ModeCorrespondence1})
	require.NoError(t, err)

	wantGame := model.NewChessState(model.ChessStateArgs{
		ID:         gameID,
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.White,
	})

	wantGame.Game.InitPieceMoves()
	wantGame.Game.ClearTables()

	assertRedisChess(t, testinfra.Redis, wantGame, cmpOptsChessState)
}
