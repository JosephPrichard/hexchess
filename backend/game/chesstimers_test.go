package game

import (
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/gameid"
	"hexchess-svc/itest"
	"hexchess-svc/utils/slogutil"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPartitionKey = gameid.GameIDPartitions()[0]

func setupTimersTest(t slogutil.TestLogger) (*TimerService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewChessTimerService(infra.Redis, testPartitionKey)

	return service, infra
}

func TestTryExpireTimers(t *testing.T) {
	service, infra := setupTimersTest(t)
	defer infra.Close()

	gameID1 := gameid.NewGameID().String()
	gameID2 := gameid.NewGameID().String()
	gameID3 := gameid.NewGameID().String()
	value1 := float64(time.Unix(1, 0).UnixMilli())
	value2 := float64(time.Unix(500, 0).UnixMilli())
	value3 := float64(time.Unix(1_001, 0).UnixMilli())

	gameTimersZSet := fmt.Sprintf("%s/game:{%s}", cache.Constants.GameTimersZSet, testPartitionKey)
	finishGamesStream := fmt.Sprintf("%s:{%s}", cache.Constants.FinishGameStreamKey, testPartitionKey)
	consumerGroup := uuid.NewString()

	// preconditions: there exists game timers with the following timer values
	for _, z := range []redis.Z{
		{Score: value1, Member: gameID1},
		{Score: value2, Member: gameID2},
		{Score: value3, Member: gameID3},
	} {
		err := infra.Redis.PrimaryClient.ZAdd(t.Context(), gameTimersZSet, z).Err()
		require.NoError(t, err)
	}

	// precondition: the finish game stream must already exist
	err := infra.Redis.PrimaryClient.XGroupCreateMkStream(t.Context(), finishGamesStream, consumerGroup, "0").Err()
	require.NoError(t, err)

	expiredKeys, err := service.TryExpireTimers(t.Context(), time.Unix(1_000, 0))
	require.NoError(t, err)

	// postcondition: expect the right keys to be expired (returned and removed from state)
	wantExpiredKeys := []any{gameID1, gameID2}
	assert.Equal(t, wantExpiredKeys, expiredKeys)

	elements, err := infra.Redis.PrimaryClient.ZRange(t.Context(), gameTimersZSet, 0, -1).Result()
	require.NoError(t, err)

	wantElements := []string{gameID3}
	assert.Equal(t, wantElements, elements)

	// postcondition: expect the right messages to be pushed to the finished game stream (to be handled by other workers)
	xArgs := &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: "1",
		Streams:  []string{finishGamesStream, ">"},
		Count:    2,
		Block:    time.Millisecond,
	}
	entries, err := infra.Redis.PrimaryClient.XReadGroup(t.Context(), xArgs).Result()
	require.NoError(t, err)

	var messages []map[string]any
	for _, e := range entries {
		for _, m := range e.Messages {
			messages = append(messages, m.Values)
		}
	}

	wantMessages := []map[string]any{
		{
			"data": fmt.Sprintf("{\"id\":\"%s\",\"replayCause\":\"TIMEOUT\"}", gameID1),
		},
		{
			"data": fmt.Sprintf("{\"id\":\"%s\",\"replayCause\":\"TIMEOUT\"}", gameID2),
		},
	}
	assert.Equal(t, wantMessages, messages)
}
