package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaderboard(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-leaderboard")
	s := State{Redis: rdb}

	// when
	for _, c := range []UpdtLbChangeSet{
		{ModeCorrespondence7, id4, 835},
		{ModeCorrespondence7, id1, 1500},
		{ModeCorrespondence7, id2, 1000},
		{ModeCorrespondence7, id3, 950},
		{ModeTimed1Plus0, id2, 1400},
		{ModeTimed1Plus0, id4, 1010},
		{ModeTimed1Plus0, id1, 900},
	} {
		require.NoError(t, s.IncrLeaderboard(ctx, c))
	}

	ranks := make([]map[string]LbRank, 0)
	leaderboards := make([]Leaderboard, 0)

	for _, id := range []int64{id1, id2, id3, id4} {
		rank, err := s.GetLeaderboardRanks(ctx, id, map[string]GameMode{"CORRESPONDENCE_7": ModeCorrespondence7, "TIMED_1+0": ModeTimed1Plus0})
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
		leaderboard, err := s.GetLeaderboard(ctx, args.mode, args.offset, args.limit)
		require.NoError(t, err)
		leaderboards = append(leaderboards, leaderboard)
	}

	// then
	wantRanks := []map[string]LbRank{
		{
			ModeCorrespondence7.String(): {Rank: 1, Score: 1500},
			ModeTimed1Plus0.String():     {Rank: 3, Score: 900}, // id1 has a value of "3" since id3 has not been lazily initialized yet
		},
		{
			ModeCorrespondence7.String(): {Rank: 2, Score: 1000},
			ModeTimed1Plus0.String():     {Rank: 1, Score: 1400},
		},
		{
			ModeCorrespondence7.String(): {Rank: 3, Score: 950},
			ModeTimed1Plus0.String():     {Rank: 3, Score: 1000},
		},
		{
			ModeCorrespondence7.String(): {Rank: 4, Score: 835},
			ModeTimed1Plus0.String():     {Rank: 2, Score: 1010},
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

func TestGetLeaderboardUsers(t *testing.T) {
	for _, test := range []struct {
		name            string
		mode            GameMode
		rankedUsers     []RankedUser
		wantLeaderboard []LbdUserEntity
		wantMissingIDs  []int64
	}{
		{
			name:        "getting leaderboard with invalid ID",
			mode:        ModeTimed1Plus0,
			rankedUsers: []RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 999999}},
			wantLeaderboard: []LbdUserEntity{
				{
					UserEntity: UserEntity{ID: 1, Username: "user1", Country: "us", JoinedOn: db.TestTimeNow},
					Elo:        1050,
					HighestElo: 1050,
					Wins:       6,
					Losses:     5,
					Winrate:    54,
					Rank:       1,
				},
			},
			wantMissingIDs: []int64{999999},
		},
		{
			name:        "getting valid leaderboard users",
			mode:        ModeCorrespondence7,
			rankedUsers: []RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 3}},
			wantLeaderboard: []LbdUserEntity{
				{
					UserEntity: UserEntity{ID: 1, Username: "user1", Country: "us", JoinedOn: db.TestTimeNow},
					Elo:        1000,
					HighestElo: 1000,
					Wins:       2,
					Losses:     2,
					Winrate:    50,
					Rank:       1,
				},
				{
					UserEntity: UserEntity{ID: 3, Username: "user3", Country: "us", JoinedOn: db.TestTimeNow},
					Elo:        900,
					HighestElo: 900,
					Rank:       2,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			pdb, closer := db.BeforePostgresTest(t, false)
			defer closer()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)
			s := State{Postgres: pdb}

			// when
			leaderboard, missingIDs, err := s.GetLeaderboardUsers(ctx, test.mode, test.rankedUsers)

			// then
			assert.Equal(t, test.wantMissingIDs, missingIDs)
			require.NoError(t, err)
			assertutil.AssertEqualIgnoring(t, test.wantLeaderboard, leaderboard)
		})
	}
}

func TestGetFuzzySearchLeaderboard(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "search-name")
	s := State{Postgres: pdb}

	// when
	users, err := s.GetFuzzySearchLeaderboard(ctx, "john", 1, 20)
	require.NoError(t, err)

	// then
	wantUsers := []LbdUserEntity{
		{UserEntity: UserEntity{ID: 8, Username: "john", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			Elo:        1500,
			HighestElo: 2000,
			Wins:       12,
			Losses:     4,
			Winrate:    66,
			Rank:       1,
		},
		{UserEntity: UserEntity{ID: 9, Username: "johnny", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			Elo:        1500,
			HighestElo: 1500,
			Wins:       5, Losses: 2,
			Winrate: 71,
			Rank:    2},
	}
	assert.Equal(t, wantUsers, users)
}
