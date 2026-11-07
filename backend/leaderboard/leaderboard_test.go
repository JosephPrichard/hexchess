package leaderboard

import (
	"context"
	"hexchess-svc/gamedata"
	"hexchess-svc/itest"
	"hexchess-svc/user"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLeaderboardTest(t slogutil.TestLogger) (*LeaderboardService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewLeaderboardService(infra.Redis, infra.Database.Querier)

	return service, infra
}

func TestLeaderboard(t *testing.T) {
	service, testinfra := setupLeaderboardTest(t)
	defer testinfra.Close()

	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)
	ctx := t.Context()

	for _, change := range []SetLbChangeSet{
		{gamedata.ModeCorrespondence7, id4, 835},
		{gamedata.ModeCorrespondence7, id1, 1500},
		{gamedata.ModeCorrespondence7, id2, 1000},
		{gamedata.ModeCorrespondence7, id3, 950},
		{gamedata.ModeTimed1Plus0, id2, 1400},
		{gamedata.ModeTimed1Plus0, id4, 1010},
		{gamedata.ModeTimed1Plus0, id1, 900},
	} {
		require.NoError(t, service.SetLeaderboard(ctx, change))
	}

	leaderboards := make([]Leaderboard, 0)

	for _, args := range []struct {
		mode   gamedata.GameMode
		offset int64
		limit  int64
	}{
		{gamedata.ModeCorrespondence7, 0, 4},
		{gamedata.ModeCorrespondence7, 1, 2},
		{gamedata.ModeTimed1Plus0, 0, 4},
	} {
		leaderboard, err := service.GetLeaderboard(ctx, args.mode, args.offset, args.limit)
		require.NoError(t, err)
		leaderboards = append(leaderboards, leaderboard)
	}

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
			RankedUsers: []user.RankedUser{{ID: id2, Rank: 1}, {ID: id4, Rank: 2}, {ID: id1, Rank: 3}},
			PageCount:   1,
		},
	}
	testutil.Equal(t, wantLeaderboards, leaderboards)
}

func TestGetFullLeaderboardUsers(t *testing.T) {
	tests := []struct {
		name            string
		mode            gamedata.GameMode
		rankedUsers     []user.RankedUser
		wantLeaderboard []user.LbdUser
		wantMissingIDs  []int64
	}{
		{
			name:        "GettingLeaderboardWithInvalidID",
			mode:        gamedata.ModeTimed1Plus0,
			rankedUsers: []user.RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 999999}},
			wantLeaderboard: []user.LbdUser{
				{
					ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow,
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
			mode:        gamedata.ModeCorrespondence7,
			rankedUsers: []user.RankedUser{{Rank: 1, ID: 1}, {Rank: 2, ID: 3}},
			wantLeaderboard: []user.LbdUser{
				{
					ID:         1,
					Username:   "user1",
					Country:    "us",
					JoinedOn:   itest.TimeNow,
					Elo:        1000,
					HighestElo: 1000,
					Wins:       2,
					Losses:     2,
					Winrate:    50,
					Rank:       1,
				},
				{
					ID:         3,
					Username:   "user3",
					Country:    "us",
					JoinedOn:   itest.TimeNow,
					Elo:        900,
					HighestElo: 900,
					Rank:       2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, testinfra := setupLeaderboardTest(t)
			defer testinfra.Close()

			ctx := context.WithValue(t.Context(), slogutil.Trace, tt.name)

			leaderboard, missingIDs, err := service.GetFullLeaderboardUsers(ctx, tt.mode, tt.rankedUsers)

			assert.Equal(t, tt.wantMissingIDs, missingIDs)
			require.NoError(t, err)
			testutil.Equal(t, tt.wantLeaderboard, leaderboard)
		})
	}
}
