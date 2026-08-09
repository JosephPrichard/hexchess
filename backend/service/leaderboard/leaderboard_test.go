package leaderboard

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/service/user"

	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/utils/alog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t alog.TestLogger, flags ...itest.TestFlag) (*LeaderboardService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewLeaderboardService(infra.Redis, infra.Querier())

	return services, infra
}

func TestLeaderboard(t *testing.T) {
	t.Parallel()

	services, testinfra := setupTest(t, itest.Redis)
	defer testinfra.Close()

	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)

	ctx := t.Context()

	for _, change := range []SetLbChangeSet{
		{model.ModeCorrespondence7, id4, 835},
		{model.ModeCorrespondence7, id1, 1500},
		{model.ModeCorrespondence7, id2, 1000},
		{model.ModeCorrespondence7, id3, 950},
		{model.ModeTimed1Plus0, id2, 1400},
		{model.ModeTimed1Plus0, id4, 1010},
		{model.ModeTimed1Plus0, id1, 900},
	} {
		require.NoError(t, services.SetLeaderboard(ctx, change))
	}

	ranks := make([]map[string]LbRank, 0)
	leaderboards := make([]Leaderboard, 0)

	for _, id := range []int64{id1, id2, id3, id4} {
		rank, err := services.GetUserLeaderboardRanks(ctx, id, map[string]model.GameMode{
			"CORRESPONDENCE_7": model.ModeCorrespondence7,
			"TIMED_1+0":        model.ModeTimed1Plus0,
		})
		require.NoError(t, err)
		ranks = append(ranks, rank)
	}
	for _, args := range []struct {
		mode   model.GameMode
		offset int64
		limit  int64
	}{
		{model.ModeCorrespondence7, 0, 4},
		{model.ModeCorrespondence7, 1, 2},
		{model.ModeTimed1Plus0, 0, 4},
	} {
		leaderboard, err := services.getLeaderboard(ctx, args.mode, args.offset, args.limit)
		require.NoError(t, err)
		leaderboards = append(leaderboards, leaderboard)
	}

	wantRanks := []map[string]LbRank{
		{
			model.ModeCorrespondence7.String(): {Rank: 1, Score: 1500},
			model.ModeTimed1Plus0.String():     {Rank: 3, Score: 900}, // id1 has a value of "3" since id3 has not been lazily initialized yet
		},
		{
			model.ModeCorrespondence7.String(): {Rank: 2, Score: 1000},
			model.ModeTimed1Plus0.String():     {Rank: 1, Score: 1400},
		},
		{
			model.ModeCorrespondence7.String(): {Rank: 3, Score: 950},
			model.ModeTimed1Plus0.String():     {Rank: 3, Score: 1000},
		},
		{
			model.ModeCorrespondence7.String(): {Rank: 4, Score: 835},
			model.ModeTimed1Plus0.String():     {Rank: 2, Score: 1010},
		},
	}
	assert.Equal(t, wantRanks, ranks)

	wantLeaderboards := []Leaderboard{
		{
			RankedUsers: []user.RankedUser{{ID: id1, Rank: 1}, {ID: id2, Rank: 2}, {ID: id3, Rank: 3}, {ID: id4, Rank: 4}},
			PageCount:   1,
		},
		{
			RankedUsers: []user.RankedUser{{ID: id2, Rank: 2}, {ID: id3, Rank: 3}},
			PageCount:   2,
		},
		{
			RankedUsers: []user.RankedUser{{ID: id2, Rank: 1}, {ID: id4, Rank: 2}, {ID: id3, Rank: 3}, {ID: id1, Rank: 4}},
			PageCount:   1,
		},
	}
	assert.Equal(t, wantLeaderboards, leaderboards)
}

func TestGetFullLeaderboardUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		mode            model.GameMode
		rankedUsers     []user.RankedUser
		wantLeaderboard []model.LbdUser
		wantMissingIDs  []int64
	}{
		{
			name:        "GettingLeaderboardWithInvalidID",
			mode:        model.ModeTimed1Plus0,
			rankedUsers: []user.RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 999999}},
			wantLeaderboard: []model.LbdUser{
				{
					User:       model.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
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
			mode:        model.ModeCorrespondence7,
			rankedUsers: []user.RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 3}},
			wantLeaderboard: []model.LbdUser{
				{
					User:       model.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
					Elo:        1000,
					HighestElo: 1000,
					Wins:       2,
					Losses:     2,
					Winrate:    50,
					Rank:       1,
				},
				{
					User:       model.User{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
					Elo:        900,
					HighestElo: 900,
					Rank:       2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupTest(t, itest.ROPostgres)
			defer testinfra.Close()

			ctx := context.WithValue(t.Context(), alog.Trace, tt.name)

			leaderboard, missingIDs, err := services.GetFullLeaderboardUsers(ctx, tt.mode, tt.rankedUsers)

			assert.Equal(t, tt.wantMissingIDs, missingIDs)
			require.NoError(t, err)
			testutil.Equal(t, tt.wantLeaderboard, leaderboard)
		})
	}
}

func TestGetFuzzySearchLeaderboard(t *testing.T) {
	t.Parallel()

	services, testinfra := setupTest(t, itest.ROPostgres)
	defer testinfra.Close()

	ctx := t.Context()

	users, err := services.GetFuzzySearchLeaderboard(ctx, "john", 1, 20)
	require.NoError(t, err)

	wantUsers := []model.LbdUser{
		{
			User:       model.User{ID: 8, Username: "john", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			Elo:        1500,
			HighestElo: 2000,
			Wins:       12,
			Losses:     4,
			Winrate:    66,
			Rank:       1,
		},
		{
			User:       model.User{ID: 9, Username: "johnny", Country: "us", Bio: "", JoinedOn: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
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
