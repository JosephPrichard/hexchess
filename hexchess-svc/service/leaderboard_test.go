package svc

import (
	"context"
	"hexchess-svc/domain"

	"hexchess-svc/util/testutil"
	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaderboard(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// testing `incrLeaderboard`, which is used to seed data for testing retreival operations
	for _, change := range []UpdtLbChangeSet{
		{domain.ModeCorrespondence7, id4, 835},
		{domain.ModeCorrespondence7, id1, 1500},
		{domain.ModeCorrespondence7, id2, 1000},
		{domain.ModeCorrespondence7, id3, 950},
		{domain.ModeTimed1Plus0, id2, 1400},
		{domain.ModeTimed1Plus0, id4, 1010},
		{domain.ModeTimed1Plus0, id1, 900},
	} {
		require.NoError(t, services.incrLeaderboard(ctx, change))
	}

	ranks := make([]map[string]LbRank, 0)
	leaderboards := make([]Leaderboard, 0)

	for _, id := range []int64{id1, id2, id3, id4} {
		rank, err := services.GetUserLeaderboardRanks(ctx, id, map[string]domain.GameMode{
			"CORRESPONDENCE_7": domain.ModeCorrespondence7,
			"TIMED_1+0":        domain.ModeTimed1Plus0,
		})
		require.NoError(t, err)
		ranks = append(ranks, rank)
	}
	for _, args := range []struct {
		mode   domain.GameMode
		offset int64
		limit  int64
	}{
		{domain.ModeCorrespondence7, 0, 4},
		{domain.ModeCorrespondence7, 1, 2},
		{domain.ModeTimed1Plus0, 0, 4},
	} {
		leaderboard, err := services.getLeaderboard(ctx, args.mode, args.offset, args.limit)
		require.NoError(t, err)
		leaderboards = append(leaderboards, leaderboard)
	}

	wantRanks := []map[string]LbRank{
		{
			domain.ModeCorrespondence7.String(): {Rank: 1, Score: 1500},
			domain.ModeTimed1Plus0.String():     {Rank: 3, Score: 900}, // id1 has a value of "3" since id3 has not been lazily initialized yet
		},
		{
			domain.ModeCorrespondence7.String(): {Rank: 2, Score: 1000},
			domain.ModeTimed1Plus0.String():     {Rank: 1, Score: 1400},
		},
		{
			domain.ModeCorrespondence7.String(): {Rank: 3, Score: 950},
			domain.ModeTimed1Plus0.String():     {Rank: 3, Score: 1000},
		},
		{
			domain.ModeCorrespondence7.String(): {Rank: 4, Score: 835},
			domain.ModeTimed1Plus0.String():     {Rank: 2, Score: 1010},
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
	t.Parallel()

	tests := []struct {
		name            string
		mode            domain.GameMode
		rankedUsers     []RankedUser
		wantLeaderboard []domain.LbdUser
		wantMissingIDs  []int64
	}{
		{
			name:        "GettingLeaderboardWithInvalidID",
			mode:        domain.ModeTimed1Plus0,
			rankedUsers: []RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 999999}},
			wantLeaderboard: []domain.LbdUser{
				{
					User:       domain.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
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
			name:        "GettingValidLeaderboardUsers",
			mode:        domain.ModeCorrespondence7,
			rankedUsers: []RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 3}},
			wantLeaderboard: []domain.LbdUser{
				{
					User:       domain.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
					Elo:        1000,
					HighestElo: 1000,
					Wins:       2,
					Losses:     2,
					Winrate:    50,
					Rank:       1,
				},
				{
					User:       domain.User{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
					Elo:        900,
					HighestElo: 900,
					Rank:       2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			services, _ := SetupServicesTest(t, Mocks{}, itest.ROPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, tt.name)

			leaderboard, missingIDs, err := services.GetLeaderboardUsers(ctx, tt.mode, tt.rankedUsers)

			assert.Equal(t, tt.wantMissingIDs, missingIDs)
			require.NoError(t, err)
			testutil.Equal(t, tt.wantLeaderboard, leaderboard)
		})
	}
}

func TestGetFuzzySearchLeaderboard(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.ROPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	users, err := services.GetFuzzySearchLeaderboard(ctx, "john", 1, 20)
	require.NoError(t, err)

	wantUsers := []domain.LbdUser{
		{
			User:       domain.User{ID: 8, Username: "john", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			Elo:        1500,
			HighestElo: 2000,
			Wins:       12,
			Losses:     4,
			Winrate:    66,
			Rank:       1,
		},
		{
			User:       domain.User{ID: 9, Username: "johnny", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			Elo:        1500,
			HighestElo: 1500,
			Wins:       5,
			Losses:     2,
			Winrate:    71,
			Rank:       2,
		},
	}
	assert.Equal(t, wantUsers, users)
}
