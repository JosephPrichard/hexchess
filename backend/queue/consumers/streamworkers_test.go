package consumers

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/config"
	"hexchess-svc/database"
	"hexchess-svc/database/testquery"
	"hexchess-svc/game"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/queue"
	"hexchess-svc/replay"
	"hexchess-svc/user"
	"hexchess-svc/utils/opt"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConsumersTest(t *testing.T) (itest.TestInfra, func(int), func()) {
	testinfra := itest.SetupIntegrationTest(t)

	ackChan := make(chan redis.XMessage)

	consumerCtx, cancelConsumer := context.WithTimeout(t.Context(), time.Second*1)

	waitStopConsumer := StartRedisConsumers(RedisConsumerConfig{
		Context:     consumerCtx,
		Database:    testinfra.Database,
		Redis:       testinfra.Redis,
		RiverClient: database.NewRiverClient(testinfra.Pool),
		Broadcaster: publishers.NewSyncBroadcaster(testinfra.Redis),

		FeatureFlagConfig: config.FeatureFlags{IsUserSearchElasticSearch: true},

		AckChan:       ackChan,
		BlockDuration: time.Millisecond * 100,
	})

	// wait until `count` messages have been consumed, then stop the consumer and wait for the consumers to stop
	waitConsumed := func(count int) {
		for range count {
			select {
			case msg := <-ackChan:
				t.Logf("received consumer message: %+v", msg)
			case <-consumerCtx.Done():
				t.Fatalf("timed out while receiving consumer message: %v", consumerCtx.Err())
			}
		}
		t.Log("sending cancel consumer signal")
		cancelConsumer()
	}
	// on shutdown, cancel the consumer, wait for the consumer to be stopped, and then shutdown infra
	customCancel := func() {
		cancelConsumer()
		waitStopConsumer()

		testinfra.Close()
	}

	return testinfra, waitConsumed, customCancel
}

func TestHandleFinishedGameEvent(t *testing.T) {
	ctx := t.Context()

	testinfra, waitConsumed, shutdownTest := setupConsumersTest(t)
	defer shutdownTest()

	whiteUser0 := user.TestUsers[0]
	blackUser1 := user.TestUsers[1]
	newGameID := gameid.NewGameID()

	inputJob := queue.FinishGameJob{
		GameID:       newGameID,
		InitialBoard: chess.NewEmptyBoard(true),
		Moves:        []chess.HistMove{},
		WhiteID:      whiteUser0.ID, // winner
		BlackID:      blackUser1.ID, // loser
		ReplayMode:   gamedata.ModeCorrespondence1.String(),
		ReplayCause:  replay.Checkmate.String(),
		ReplayResult: replay.WhiteWin.String(),
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
	assertBroadcasts := game.ExpectBroadcastGames(t, testinfra.Redis, newGameID, wantGameOutputs)

	err := producers.ProduceFinishGame(ctx, testinfra.Redis.PrimaryClient, inputJob)
	require.NoError(t, err)

	waitConsumed(1)

	userElos, err := testinfra.Querier.SelectUserModeElosByIDs(ctx, testquery.SelectUserModeElosByIDsParams{
		ID:   []int64{whiteUser0.ID, blackUser1.ID},
		Mode: testquery.ModeEnumCORRESPONDENCE1,
	})
	require.NoError(t, err)

	wantUserElos := []testquery.UserModeElo{
		{UserID: whiteUser0.ID, Mode: testquery.ModeEnumCORRESPONDENCE1, Elo: 1015, HighestElo: 1015, Wins: 1},
		{UserID: blackUser1.ID, Mode: testquery.ModeEnumCORRESPONDENCE1, Elo: 985, HighestElo: 1000, Losses: 1},
	}
	assert.Equal(t, wantUserElos, userElos)

	assertBroadcasts()
}

func TestHandleUpdtGameEvent(t *testing.T) {
	ctx := t.Context()

	testinfra, waitConsumed, shutdownTest := setupConsumersTest(t)
	defer shutdownTest()

	whiteUser0 := user.TestUsers[0]
	blackUser1 := user.TestUsers[1]
	gameID := gameid.NewGameID()

	job := queue.UpdtGameMetadataJob{
		GameID:      gameID,
		WhitePlayer: opt.Some(whiteUser0.ID),
		BlackPlayer: opt.Some(blackUser1.ID),
		Mode:        gamedata.ModeCorrespondence1.String(),
		FirstColor:  gamedata.Random.String(),
	}

	err := producers.ProduceUpdtGameMetadata(ctx, testinfra.Redis.PrimaryClient, job)
	require.NoError(t, err)

	waitConsumed(1)

	gameRow, err := testinfra.Querier.SelectGameMeta(ctx, gameID.String())
	require.NoError(t, err)

	wantGameRow := testquery.GamesMetadatum{
		Ordering: 4,
		GameID:   gameID.String(),
		Mode:     "CORRESPONDENCE_1",
		WhiteID:  pgtype.Int8{Int64: whiteUser0.ID, Valid: true},
		BlackID:  pgtype.Int8{Int64: blackUser1.ID, Valid: true},
	}
	var gameRowCmpOpts = cmpopts.IgnoreFields(testquery.GamesMetadatum{}, "UpdatedOn")
	testutil.Equal(t, wantGameRow, gameRow, gameRowCmpOpts)
}
