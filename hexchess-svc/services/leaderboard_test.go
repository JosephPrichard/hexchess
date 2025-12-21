package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
)

func TestLeaderboard(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)

	ctx := context.WithValue(t.Context(), util.Trace, "testing-leaderboard")

	// when
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeCorrespondence7, UpdtLbChangeSet{id4, 835}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeCorrespondence7, UpdtLbChangeSet{id1, 1500}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeCorrespondence7, UpdtLbChangeSet{id2, 1000}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeCorrespondence7, UpdtLbChangeSet{id3, 950}))

	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeTimed1Plus0, UpdtLbChangeSet{id2, 1400}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeTimed1Plus0, UpdtLbChangeSet{id4, 1010}))
	//assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeTimed1Plus0, UpdtLbChangeSet{id3, 1000}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, ModeTimed1Plus0, UpdtLbChangeSet{id1, 920}))

	modes := []GameMode{ModeCorrespondence7, ModeTimed1Plus0}
	ranks1, err := GetLeaderboardRanks(ctx, rdb, id1, modes)
	assert.NoError(t, err)
	ranks2, err := GetLeaderboardRanks(ctx, rdb, id2, modes)
	assert.NoError(t, err)
	ranks3, err := GetLeaderboardRanks(ctx, rdb, id3, modes)
	assert.NoError(t, err)
	ranks4, err := GetLeaderboardRanks(ctx, rdb, id4, modes)
	assert.NoError(t, err)

	leaderboard1, err := GetLeaderboard(ctx, rdb, ModeCorrespondence7, 0, 4)
	assert.NoError(t, err)

	leaderboard2, err := GetLeaderboard(ctx, rdb, ModeCorrespondence7, 1, 2)
	assert.NoError(t, err)

	leaderboard3, err := GetLeaderboard(ctx, rdb, ModeTimed1Plus0, 0, 4)
	assert.NoError(t, err)

	// then
	assert.Equal(t, map[GameMode]LbRank{
		ModeCorrespondence7: {Rank: 1, Score: 1500},
		ModeTimed1Plus0:     {Rank: 4, Score: 920},
	}, ranks1)
	assert.Equal(t, map[GameMode]LbRank{
		ModeCorrespondence7: {Rank: 2, Score: 1000},
		ModeTimed1Plus0:     {Rank: 1, Score: 1400},
	}, ranks2)
	assert.Equal(t, map[GameMode]LbRank{
		ModeCorrespondence7: {Rank: 3, Score: 950},
		ModeTimed1Plus0:     {Rank: 3, Score: 1000},
	}, ranks3)
	assert.Equal(t, map[GameMode]LbRank{
		ModeCorrespondence7: {Rank: 4, Score: 835},
		ModeTimed1Plus0:     {Rank: 2, Score: 1010},
	}, ranks4)

	wantLeaderboard1 := Leaderboard{
		Users:     []RankedUser{{ID: id1, Rank: 1}, {ID: id2, Rank: 2}, {ID: id3, Rank: 3}, {ID: id4, Rank: 4}},
		PageCount: 1,
	}
	wantLeaderboard2 := Leaderboard{
		Users:     []RankedUser{{ID: id2, Rank: 2}, {ID: id3, Rank: 3}},
		PageCount: 2,
	}
	wantLeaderboard3 := Leaderboard{
		Users:     []RankedUser{{ID: id2, Rank: 1}, {ID: id4, Rank: 2}, {ID: id3, Rank: 3}, {ID: id1, Rank: 4}},
		PageCount: 1,
	}
	assert.Equal(t, wantLeaderboard1, leaderboard1)
	assert.Equal(t, wantLeaderboard2, leaderboard2)
	assert.Equal(t, wantLeaderboard3, leaderboard3)
}
