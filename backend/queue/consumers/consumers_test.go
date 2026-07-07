package consumers

import (
	"context"
	"hexchess-lib/async"
	"hexchess-lib/optional"
	"hexchess-lib/testutil"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	svc "hexchess-svc/service"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(sqlc.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestHandleAdvanceTournamentEvent(t *testing.T) {
	ctx := t.Context()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
	})

	tournamentKey := itest.Tournament2ScheduledKnockoutKey

	err := producers.PublishAdvanceTournamentEvent(ctx, testinfra.Querier, tournamentKey, time.Time{})
	require.NoError(t, err)

	eventGateway := EventGateway{services: services}
	queue := PostgresConsumer{
		ctx:         ctx,
		pdb:         testinfra.DB,
		consumeFunc: eventGateway.HandleAdvanceTournamentEvent,
		PostgresConfig: PostgresConfig{
			EventKind:    sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
			PollInterval: time.Microsecond,
			PollCount:    1,
			MaxEvents:    1,
		},
	}

	queue.Consume()

	wantMatches := []sqlc.TournamentMatch{
		// tournament has 2 rounds with join order of [1,2,3,4], so starting the tournament creates 2 rounds wso the matches go 1-2, 3-4
		{
			TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
			Round:         1,
			WhiteID:       3,
			BlackID:       4,
		},
		{
			TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
			Round:         1,
			WhiteID:       5,
			BlackID:       6,
		},
	}

	matches, err := testinfra.Querier.SelectMatches(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	require.NoError(t, err)
	testutil.Equal(t, wantMatches, matches, sqlcTournamentMatchCmpOpts)
}

func TestHandleFinishedGameEvent(t *testing.T) {
	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
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
		WhitePlayer:  model.PlayerState{ID: whiteUser0.ID, Present: true}, // winner
		BlackPlayer:  model.PlayerState{ID: blackUser1.ID, Present: true}, // loser
		ReplayMode:   model.ModeCorrespondence1,
		ReplayCause:  model.Checkmate,
		ReplayResult: model.WhiteWin,
	}

	publisher := producers.NewPublisher(testinfra.Redis)
	err := publisher.PublishFinishGameEvent(ctx, testinfra.Redis.Primary, finishedGame)
	require.NoError(t, err)

	eventGateway := EventGateway{services: services}

	queue := RedisConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.Primary,
		consumeFunc: eventGateway.HandleFinishedGameEvent,
		dispatcher:  async.SyncDispatcher{},

		RedisConfig: RedisConfig{
			PollCount:     1,
			MaxEvents:     1,
			BlockDuration: time.Millisecond,
			StreamKey:     testinfra.Redis.FinishGameStreamKey,
			ConsumerGroup: testinfra.Redis.FinishGameConsumerGroup,
			PartitionKeys: []string{string(partitionID)},
		},
	}

	queue.ConsumePartition(string(partitionID))

	userElos, err := testinfra.Querier.SelectUserModeElosByIDs(ctx, sqlc.SelectUserModeElosByIDsParams{
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
	ctx := t.Context()

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
		Entropy:     &svc.StableEntropySource{CurrTime: itest.TimeNow},
	})

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	gameID := model.NewGameID()
	partitionID := gameID.Partition()

	updtGame := model.GameMetadataUpdt{
		GameID:      gameID,
		WhitePlayer: optional.Just(whiteUser0.ID),
		BlackPlayer: optional.Just(blackUser1.ID),
		Mode:        model.ModeCorrespondence1,
		FirstColor:  model.Random,
	}

	publisher := producers.NewPublisher(testinfra.Redis)
	err := publisher.PublishUpdtGameEvent(ctx, testinfra.Redis.Primary, updtGame)
	require.NoError(t, err)

	eventGateway := EventGateway{services: services}

	queue := RedisConsumer{
		ctx:         consumerCtx,
		cancel:      cancel,
		redis:       testinfra.Redis.Primary,
		consumeFunc: eventGateway.HandleUpdtGameEvent,
		dispatcher:  async.SyncDispatcher{},

		RedisConfig: RedisConfig{
			PollCount:     1,
			MaxEvents:     1,
			BlockDuration: time.Millisecond,
			StreamKey:     testinfra.Redis.UpdtGameMetaStreamKey,
			ConsumerGroup: testinfra.Redis.UpdtGameMetaConsumerGroup,
			PartitionKeys: []string{string(partitionID)},
		},
	}
	queue.ConsumePartition(string(partitionID))

	gameRow, err := testinfra.Querier.SelectGameMeta(ctx, gameID.String())
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
