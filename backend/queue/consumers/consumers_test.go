package consumers

import (
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/testutil"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	svc "hexchess-svc/service"
	"testing"
	"time"
)

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(sqlc.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestHandleAdvanceTournamentEvent(t *testing.T) {
	ctx := t.Context()

	testinfra := itest.SetupTestInfra(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:      testinfra.DB,
		Querier: testinfra.Querier,
		Redis:   testinfra.Redis,
	})

	tournamentKey := itest.Tournament2ScheduledKnockoutKey

	err := producers.PublishAdvanceTournamentEvent(ctx, testinfra.Querier, tournamentKey, time.Time{})
	require.NoError(t, err)

	queue := PostgresConsumer{
		ctx: ctx,

		pdb:     testinfra.DB,
		entropy: &svc.StableEntropySource{CurrTime: itest.TimeNow},

		kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
		pollInterval: time.Microsecond,
		pollCount:    1,
		maxEvents:    1,
		fn:           HandleAdvanceTournamentEvent(services),
	}

	err = queue.Consume()
	require.NoError(t, err)

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

	testinfra := itest.SetupTestInfra(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Querier:     testinfra.Querier,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NoopBroadcaster{},
	})

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	newGameID := uuid.NewString()

	finishedGame := model.FinishedGame{
		GameID:       newGameID,
		Board:        chess.MakeEmptyBoard(true),
		Moves:        []chess.HistMove{},
		WhitePlayer:  model.PlayerState{ID: whiteUser0.ID, Present: true}, // winner
		BlackPlayer:  model.PlayerState{ID: blackUser1.ID, Present: true}, // loser
		ReplayMode:   model.ModeCorrespondence1,
		ReplayCause:  model.Checkmate,
		ReplayResult: model.WhiteWin,
	}

	publisher := producers.MakePublisher(testinfra.Redis)
	err := publisher.PublishFinishGameEvent(ctx, testinfra.Redis.GameStore, finishedGame)
	require.NoError(t, err)

	queue := RedisConsumer{
		ctx:   ctx,
		redis: testinfra.Redis.GameStore,

		concurrency:   1,
		maxEvents:     1,
		streamKey:     testinfra.Redis.FinishGameStreamKey,
		consumerGroup: testinfra.Redis.FinishGameConsumerGroup,

		fn: HandleFinishedGameEvent(services),
	}

	err = queue.Consume()
	require.NoError(t, err)

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

	testinfra := itest.SetupTestInfra(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Querier:     testinfra.Querier,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NoopBroadcaster{},
		Entropy:     &svc.StableEntropySource{CurrTime: itest.TimeNow},
	})

	whiteUser0 := itest.TestUser[0]
	blackUser1 := itest.TestUser[1]
	gameID := uuid.NewString()

	updtGame := model.GameMetadataUpdt{
		GameID:      gameID,
		WhitePlayer: enum.Just(whiteUser0.ID),
		BlackPlayer: enum.Just(blackUser1.ID),
		Mode:        model.ModeCorrespondence1,
		FirstColor:  model.Random,
	}

	publisher := producers.MakePublisher(testinfra.Redis)
	err := publisher.PublishUpdtGameEvent(ctx, testinfra.Redis.GameStore, updtGame)
	require.NoError(t, err)

	queue := RedisConsumer{
		ctx:   ctx,
		redis: testinfra.Redis.GameStore,

		concurrency:   1,
		maxEvents:     1,
		streamKey:     testinfra.Redis.UpdtGameMetaStreamKey,
		consumerGroup: testinfra.Redis.UpdtGameMetaConsumerGroup,

		fn: HandleUpdtGameEvent(services),
	}

	err = queue.Consume()
	require.NoError(t, err)

	gameRow, err := testinfra.Querier.SelectGameMeta(ctx, gameID)
	require.NoError(t, err)

	wantGameRow := sqlc.GamesMetadatum{
		Ordering:  4,
		GameID:    gameID,
		Mode:      "CORRESPONDENCE_1",
		WhiteID:   pgtype.Int8{Int64: whiteUser0.ID, Valid: true},
		BlackID:   pgtype.Int8{Int64: blackUser1.ID, Valid: true},
		UpdatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
	}
	testutil.Equal(t, wantGameRow, gameRow)
}
