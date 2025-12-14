package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"math"
	"math/rand"
	"testing"
)

func TestLeaderboard(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := int64(rand.Intn(math.MaxInt64))
	id2 := int64(rand.Intn(math.MaxInt64))
	id3 := int64(rand.Intn(math.MaxInt64))
	id4 := int64(rand.Intn(math.MaxInt64))

	ctx := context.WithValue(context.Background(), util.Trace, "testing-leaderboard")

	// when
	assert.NoError(t, IncrLeaderboard(ctx, rdb, UpdtLbChangeSet{id1, 1500}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, UpdtLbChangeSet{id2, 1000}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, UpdtLbChangeSet{id3, 950}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, UpdtLbChangeSet{id4, 835}))

	rank1, err := GetLeaderboardRank(ctx, rdb, id1)
	assert.NoError(t, err)
	rank2, err := GetLeaderboardRank(ctx, rdb, id2)
	assert.NoError(t, err)
	rank3, err := GetLeaderboardRank(ctx, rdb, id3)
	assert.NoError(t, err)
	rank4, err := GetLeaderboardRank(ctx, rdb, id4)
	assert.NoError(t, err)

	leaderboard1, err := GetLeaderboard(ctx, rdb, 0, 4)
	assert.NoError(t, err)

	leaderboard2, err := GetLeaderboard(ctx, rdb, 1, 2)
	assert.NoError(t, err)

	// then
	assert.Equal(t, int64(1), rank1)
	assert.Equal(t, int64(2), rank2)
	assert.Equal(t, int64(3), rank3)
	assert.Equal(t, int64(4), rank4)

	expectedLeaderboard1 := Leaderboard{
		Users:     []RankedUser{{ID: id1, Rank: 1}, {ID: id2, Rank: 2}, {ID: id3, Rank: 3}, {ID: id4, Rank: 4}},
		PageCount: 1,
	}

	expectedLeaderboard2 := Leaderboard{
		Users:     []RankedUser{{ID: id2, Rank: 2}, {ID: id3, Rank: 3}},
		PageCount: 2,
	}
	assert.Equal(t, expectedLeaderboard1, leaderboard1)
	assert.Equal(t, expectedLeaderboard2, leaderboard2)
}
