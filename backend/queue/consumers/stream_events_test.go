package consumers

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleFinishedGameEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		Database:    testinfra.Database,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
	})

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	newGameID := model.NewGameID()
	partitionID := newGameID.Partition()

	finishedGame := model.FinishedGame{
		GameID:       newGameID,
		Board:        chess.NewEmptyBoard(true),
		Moves:        []chess.HistMove{},
		WhitePlayer:  whiteUser0.ID, // winner
		BlackPlayer:  blackUser1.ID, // loser
		ReplayMode:   model.ModeCorrespondence1,
		ReplayCause:  model.Checkmate,
		ReplayResult: model.WhiteWin,
	}

	publisher := producers.NewPublisher(testinfra.Redis)
	err := publisher.ProduceFinishGame(ctx, testinfra.Redis.PrimaryClient, finishedGame)
	require.NoError(t, err)

	handler := FinishedGameHandler{services: services}

	consumer := StreamConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.PrimaryClient,
		consumeFunc: handler.Handle,

		querier:    testinfra.Database.Querier(),
		dispatcher: async.SyncDispatcher{},

		pollCount:     1,
		maxEvents:     1,
		blockDuration: time.Millisecond,
		streamKey:     testinfra.Redis.FinishGameStreamKey,
		consumerGroup: testinfra.Redis.FinishGameConsumerGroup,
		partitionKeys: []string{string(partitionID)},
	}

	consumer.ConsumePartition(string(partitionID))

	userElos, err := testinfra.PrimaryQuerier.SelectUserModeElosByIDs(ctx, sqlc.SelectUserModeElosByIDsParams{
		ID:   []int64{whiteUser0.ID, blackUser1.ID},
		Mode: "CORRESPONDENCE_1",
	})
	require.NoError(t, err)

	wantUserElos := []sqlc.SelectUserModeElosByIDsRow{
		{UserID: whiteUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},
		{UserID: blackUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1},
	}
	assert.Equal(t, wantUserElos, userElos)
}

func TestHandleUpdtGameEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		Database:    testinfra.Database,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
		Entropy:     &entropy.StableSource{CurrTime: itest.TimeNow},
	})

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	gameID := model.NewGameID()
	partitionID := gameID.Partition()

	updtGame := model.GameMetadataUpdt{
		GameID:      gameID,
		WhitePlayer: whiteUser0.ID,
		BlackPlayer: blackUser1.ID,
		Mode:        model.ModeCorrespondence1,
		FirstColor:  model.Random,
	}

	publisher := producers.NewPublisher(testinfra.Redis)
	err := publisher.ProduceUpdtGameMetadata(ctx, testinfra.Redis.PrimaryClient, updtGame)
	require.NoError(t, err)

	handler := UpdtGameMetadataHandler{services: services}

	consumer := StreamConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.PrimaryClient,
		consumeFunc: handler.Handle,

		querier:    testinfra.Database.Querier(),
		dispatcher: async.SyncDispatcher{},

		pollCount:     1,
		maxEvents:     1,
		blockDuration: time.Millisecond,
		streamKey:     testinfra.Redis.UpdtGameMetaStreamKey,
		consumerGroup: testinfra.Redis.UpdtGameMetaConsumerGroup,
		partitionKeys: []string{string(partitionID)},
	}
	consumer.ConsumePartition(string(partitionID))

	gameRow, err := testinfra.PrimaryQuerier.SelectGameMeta(ctx, gameID.String())
	require.NoError(t, err)

	wantGameRow := sqlc.GamesMetadatum{
		Ordering:  4,
		GameID:    gameID.String(),
		Mode:      "CORRESPONDENCE_1",
		WhiteID:   pgtype.Int8{Int64: whiteUser0.ID, Valid: true},
		BlackID:   pgtype.Int8{Int64: blackUser1.ID, Valid: true},
		UpdatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
	}
	testutil.Equal(t, wantGameRow, gameRow)
}
