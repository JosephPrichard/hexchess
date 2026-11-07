package game

import (
	"hexchess-svc/database/testquery"
	"hexchess-svc/gamedata"
	"hexchess-svc/itest"
	"hexchess-svc/player"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/user"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameMetadataTest(t slogutil.TestLogger) (*ChessMetadataService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewChessMetadataService(infra.Database, &entropy.StableSource{CurrTime: itest.TimeNow}, publishers.NewSyncBroadcaster(infra.Redis))

	return service, infra
}

func TestGetGameMetadata(t *testing.T) {
	service, testinfra := setupGameMetadataTest(t)
	defer testinfra.Close()

	resp, err := service.GetGameMetadata(t.Context(), player.NewPlayer(2, "", ""), opt.None[int64](), 10)
	require.NoError(t, err)

	wantAllChessMetas := []ChessMeta{
		{Ordering: 3, GameID: itest.GameID3, Mode: gamedata.ModeCorrespondence1},
		{Ordering: 2, GameID: itest.GameID2, Mode: gamedata.ModeCorrespondence1},
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: opt.Some(user.User{ID: 2, Username: "user2", Country: "us"}),
			Mode:        gamedata.ModeCorrespondence1,
		},
	}
	wantSelfChessMetas := []ChessMeta{
		{
			Ordering:    1,
			GameID:      itest.GameID1,
			BlackPlayer: opt.Some(user.User{ID: 2, Username: "user2", Country: "us"}),
			Mode:        gamedata.ModeCorrespondence1,
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

	err := service.UpdateGameMetadata(ctx, UpdtGameMetadataEvent{
		GameID:      itest.GameID1,
		WhitePlayer: opt.Some(int64(1)),
		Mode:        gamedata.ModeCorrespondence1,
		FirstColor:  gamedata.Random,
	})
	require.NoError(t, err)

	gameMeta, err := testinfra.Querier.SelectGameMeta(ctx, itest.GameID1.String())
	require.NoError(t, err)

	actualGameMeta := testquery.GamesMetadatum{
		GameID:    gameMeta.GameID,
		Ordering:  5,
		Mode:      "CORRESPONDENCE_1",
		WhiteID:   pgtype.Int8{Int64: 1, Valid: true},
		UpdatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
	}
	testutil.Equal(t, actualGameMeta, gameMeta)
}
