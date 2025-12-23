package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	for _, c := range []UpdtLbChangeSet{
		{ModeCorrespondence7, id4, 835},
		{ModeCorrespondence7, id1, 1500},
		{ModeCorrespondence7, id2, 1000},
		{ModeCorrespondence7, id3, 950},
		{ModeTimed1Plus0, id2, 1400},
		{ModeTimed1Plus0, id4, 1010},
		// {ModeTimed1Plus0, id3, 1000},
		{ModeTimed1Plus0, id1, 900},
	} {
		require.NoError(t, IncrLeaderboard(ctx, rdb, c))
	}

	ranks := make([]map[GameMode]LbRank, 0)
	leaderboards := make([]Leaderboard, 0)

	for _, id := range []int64{id1, id2, id3, id4} {
		rank, err := GetLeaderboardRanks(ctx, rdb, id, []GameMode{ModeCorrespondence7, ModeTimed1Plus0})
		require.NoError(t, err)
		ranks = append(ranks, rank)
	}
	for _, args := range []struct {
		mode   GameMode
		offset int64
		limit  int64
	}{
		{ModeCorrespondence7, 0, 4},
		{ModeCorrespondence7, 1, 2},
		{ModeTimed1Plus0, 0, 4},
	} {
		leaderboard, err := GetLeaderboard(ctx, rdb, args.mode, args.offset, args.limit)
		require.NoError(t, err)
		leaderboards = append(leaderboards, leaderboard)
	}

	// then
	wantRanks := []map[GameMode]LbRank{
		{
			ModeCorrespondence7: {Rank: 1, Score: 1500},
			ModeTimed1Plus0:     {Rank: 3, Score: 900}, // id1 has a value of "3" since id3 has not been lazily initialized yet
		},
		{
			ModeCorrespondence7: {Rank: 2, Score: 1000},
			ModeTimed1Plus0:     {Rank: 1, Score: 1400},
		},
		{
			ModeCorrespondence7: {Rank: 3, Score: 950},
			ModeTimed1Plus0:     {Rank: 3, Score: 1000},
		},
		{
			ModeCorrespondence7: {Rank: 4, Score: 835},
			ModeTimed1Plus0:     {Rank: 2, Score: 1010},
		},
	}
	assert.Equal(t, wantRanks, ranks)

	wantLeaderboards := []Leaderboard{
		{
			RankedUsers: []RankedUser{{ID: id1, Rank: 1}, {ID: id2, Rank: 2}, {ID: id3, Rank: 3}, {ID: id4, Rank: 4}},
			PageCount:   1,
		},
		{
			RankedUsers: []RankedUser{{ID: id2, Rank: 2}, {ID: id3, Rank: 3}},
			PageCount:   2,
		},
		{
			RankedUsers: []RankedUser{{ID: id2, Rank: 1}, {ID: id4, Rank: 2}, {ID: id3, Rank: 3}, {ID: id1, Rank: 4}},
			PageCount:   1,
		},
	}
	assert.Equal(t, wantLeaderboards, leaderboards)
}
