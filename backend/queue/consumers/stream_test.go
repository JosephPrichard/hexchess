package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/database/query"
	"hexchess-svc/pb"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/opt"

	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleFinishedGameEvent(t *testing.T) {
	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t)
	defer testinfra.Close()

	worker := NewFinishedGameWorker(testinfra.Database, testinfra.Redis, &producers.NoopRiverClient{}, pubsub.NewSyncBroadcaster(testinfra.Redis))

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	newGameID := model.NewGameID()
	partitionID := newGameID.Partition()

	finishedGame := model.FinishGameEvent{
		GameID:       newGameID,
		Board:        chess.NewEmptyBoard(true),
		Moves:        []chess.HistMove{},
		WhitePlayer:  whiteUser0.ID, // winner
		BlackPlayer:  blackUser1.ID, // loser
		ReplayMode:   model.ModeCorrespondence1,
		ReplayCause:  model.Checkmate,
		ReplayResult: model.WhiteWin,
	}

	wantGameOutputs := []*pb.GameOutput{
		{
			GameId: newGameID.String(),
			Value: &pb.GameOutput_Replay{Replay: &pb.ReplayOutput{
				BlackCountry: "us",
				BlackElo:     985,
				BlackEloDiff: -15,
				BlackId:      2,
				BlackName:    "user2",
				Cause:        "CHECKMATE",
				LoseEloDiff:  -15,
				Mode:         "CORRESPONDENCE_1",
				Result:       "WHITE_WINS",
				WhiteCountry: "us",
				WhiteElo:     1015,
				WhiteEloDiff: 15,
				WhiteId:      1,
				WhiteName:    "user1",
				WinEloDiff:   15,
			}},
		},
	}
	assertBroadcasts := pubsub.ExpectBroadcastGames(t, testinfra.Redis, newGameID, wantGameOutputs)

	err := producers.ProduceFinishGame(ctx, testinfra.Redis.PrimaryClient, finishedGame)
	require.NoError(t, err)

	consumer := StreamConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.PrimaryClient,
		consumeFunc: worker.Handle,

		querier:    testinfra.QuerierMutator(),
		dispatcher: async.SyncDispatcher{},

		pollCount:     1,
		maxEvents:     1,
		blockDuration: time.Millisecond,
		streamKey:     cache.Constants.FinishGameStreamKey,
		consumerGroup: cache.Constants.FinishGameConsumerGroup,
		partitionKeys: []string{string(partitionID)},
	}

	consumer.ConsumePartition(string(partitionID))

	userElos, err := testinfra.Querier().SelectUserModeElosByIDs(ctx, query.SelectUserModeElosByIDsParams{
		ID:   []int64{whiteUser0.ID, blackUser1.ID},
		Mode: "CORRESPONDENCE_1",
	})
	require.NoError(t, err)

	wantUserElos := []query.SelectUserModeElosByIDsRow{
		{UserID: whiteUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},
		{UserID: blackUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1},
	}
	assert.Equal(t, wantUserElos, userElos)

	assertBroadcasts()
}

func TestHandleUpdtGameEvent(t *testing.T) {
	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t)
	defer testinfra.Close()

	worker := NewUpdtGameMetadataWorker(testinfra.Database, &entropy.StableSource{CurrTime: itest.TimeNow}, pubsub.NewSyncBroadcaster(testinfra.Redis))

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	gameID := model.NewGameID()
	partitionID := gameID.Partition()

	updtGame := model.UpdtGameMetadataEvent{
		GameID:      gameID,
		WhitePlayer: opt.Some(whiteUser0.ID),
		BlackPlayer: opt.Some(blackUser1.ID),
		Mode:        model.ModeCorrespondence1,
		FirstColor:  model.Random,
	}

	err := producers.ProduceUpdtGameMetadata(ctx, testinfra.Redis.PrimaryClient, updtGame)
	require.NoError(t, err)

	consumer := StreamConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.PrimaryClient,
		consumeFunc: worker.Handle,

		querier:    testinfra.QuerierMutator(),
		dispatcher: async.SyncDispatcher{},

		pollCount:     1,
		maxEvents:     1,
		blockDuration: time.Millisecond,
		streamKey:     cache.Constants.UpdtGameMetaStreamKey,
		consumerGroup: cache.Constants.UpdtGameMetaConsumerGroup,
		partitionKeys: []string{string(partitionID)},
	}
	consumer.ConsumePartition(string(partitionID))

	gameRow, err := testinfra.Querier().SelectGameMeta(ctx, gameID.String())
	require.NoError(t, err)

	wantGameRow := query.GamesMetadatum{
		Ordering:  4,
		GameID:    gameID.String(),
		Mode:      "CORRESPONDENCE_1",
		WhiteID:   pgtype.Int8{Int64: whiteUser0.ID, Valid: true},
		BlackID:   pgtype.Int8{Int64: blackUser1.ID, Valid: true},
		UpdatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
	}
	testutil.Equal(t, wantGameRow, gameRow)
}
