package service

import (
	"hexchess-svc/database/query"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameMetadataTest(t slogutil.TestLogger) (*ChessMetaService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewChessMetaService(infra.Database, &entropy.StableSource{CurrTime: itest.TimeNow}, pubsub.NewSyncBroadcaster(infra.Redis))

	return services, infra
}

func TestGetGameMetadata(t *testing.T) {
	service, testinfra := setupGameMetadataTest(t)
	defer testinfra.Close()

	resp, err := service.GetGameMetadata(t.Context(), model.NewPlayer(2, "", ""), opt.None[int64](), 10)
	require.NoError(t, err)

	wantAllChessMetas := []model.ChessMeta{
		{Ordering: 3, GameID: itest.GameID3, Mode: model.ModeCorrespondence1},
		{Ordering: 2, GameID: itest.GameID2, Mode: model.ModeCorrespondence1},
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: opt.Some(model.User{ID: 2, Username: "user2", Country: "us"}),
			Mode:        model.ModeCorrespondence1,
		},
	}
	wantSelfChessMetas := []model.ChessMeta{
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: opt.Some(model.User{ID: 2, Username: "user2", Country: "us"}),
			Mode:        model.ModeCorrespondence1,
		},
	}
	assert.Equal(t, wantAllChessMetas, resp.ChessMetas)
	assert.Equal(t, wantSelfChessMetas, resp.UserChessMetas)
}

func TestCountGameMetadata(t *testing.T) {
	service, testinfra := setupGameMetadataTest(t)
	defer testinfra.Close()

	count, err := service.GetGameMetadataCount(t.Context())
	require.NoError(t, err)

	assert.Equal(t, int64(3), count)
}

func TestUpdateGameMetadata(t *testing.T) {
	ctx := t.Context()

	service, testinfra := setupGameMetadataTest(t)
	defer testinfra.Close()

	err := service.UpdateGameMetadata(ctx, model.UpdtGameMetadataEvent{
		GameID:      itest.GameID1,
		WhitePlayer: opt.Some(int64(1)),
		Mode:        model.ModeCorrespondence1,
		FirstColor:  model.Random,
	})
	require.NoError(t, err)

	gameMeta, err := testinfra.QuerierMutator().SelectGameMeta(ctx, itest.GameID1.String())
	require.NoError(t, err)

	actualGameMeta := query.GamesMetadatum{
		GameID:    gameMeta.GameID,
		Ordering:  5,
		Mode:      "CORRESPONDENCE_1",
		WhiteID:   pgtype.Int8{Int64: 1, Valid: true},
		UpdatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
	}
	testutil.Equal(t, actualGameMeta, gameMeta)
}
