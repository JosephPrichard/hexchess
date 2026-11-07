package trigger

import (
	"context"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/gameid"
	"hexchess-svc/itest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPartitionKey = gameid.GameIDPartitions()[0]

func TestExecute(t *testing.T) {
	infra := itest.SetupIntegrationTest(t)
	defer infra.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()

	gameID1 := gameid.NewGameID().String()
	value1 := float64(time.Unix(1, 0).UnixMilli())

	gameTimersZSet := fmt.Sprintf("%s/game:{%s}", cache.Constants.GameTimersZSet, testPartitionKey)

	// preconditions: there exists game timers with the following timer values
	for _, z := range []redis.Z{
		{Score: value1, Member: gameID1},
	} {
		err := infra.Redis.PrimaryClient.ZAdd(t.Context(), gameTimersZSet, z).Err()
		require.NoError(t, err)
	}

	Execute(ctx, TriggerConfig{RedisClient: infra.Redis, Period: time.Millisecond * 5})

	// postcondition: expect the right keys to be expired (removed from state)
	elements, err := infra.Redis.PrimaryClient.ZRange(t.Context(), gameTimersZSet, 0, -1).Result()
	require.NoError(t, err)

	// postcondition: expect there to be no elements (all removed)
	assert.Equal(t, make([]string, 0), elements)
}
