package service

import (
	"hexchess-svc/cache"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setupPersonaService(t slogutil.TestLogger) (*PersonaService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewPersonaService(infra.Database, infra.Redis)

	return service, infra
}

func TestGetPersona(t *testing.T) {
	service, testinfra := setupPersonaService(t)
	defer testinfra.Close()

	persona, err := service.GetPersona(t.Context(), 1, 10)
	require.NoError(t, err)

	wantPersona := model.Persona{
		User: itest.TestUser[0],
		Stats: model.UserStats{
			TotalWins:    17,
			TotalLosses:  14,
			AvgElo:       1025,
			HighestElo:   1050,
			TotalWinrate: 53,
			ModeStats: []model.ModeStats{
				{Mode: model.ModeCorrespondence7, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
				{Mode: model.ModeTimed3Plus2, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
				{Mode: model.ModeTimed15Plus10, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
				{Mode: model.ModeTimed1Plus0, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
			},
		},
		ReplayList: []model.FullReplay{
			itest.TestReplays[4],
			itest.TestReplays[3],
			itest.TestReplays[2],
			itest.TestReplays[0],
		},
	}

	cmpOptsModeStats := cmpopts.IgnoreFields(model.ModeStats{}, "Score")
	testutil.Equal(t, persona, wantPersona, cmpOptsModeStats)
}

func TestGetPersona_InvalidUserID(t *testing.T) {
	service, testinfra := setupPersonaService(t)
	defer testinfra.Close()

	_, err := service.GetPersona(t.Context(), 54915, 10)
	leafErr := leafError(err)

	wantErr := ErrUserNotFound
	require.Equal(t, wantErr, leafErr)
}

func TestGetPersona_GetsPersonaLeaderboardRanks(t *testing.T) {
	service, testinfra := setupPersonaService(t)
	defer testinfra.Close()

	ctx := t.Context()

	userID1 := int64(1)
	userID2 := int64(2)
	userID3 := int64(3)
	userID4 := int64(4)

	for _, change := range []struct {
		Mode model.GameMode
		ID   int64
		Elo  float64
	}{
		{Mode: model.ModeCorrespondence7, ID: userID1, Elo: 1000},
		{Mode: model.ModeCorrespondence7, ID: userID3, Elo: 900},
		{Mode: model.ModeCorrespondence7, ID: userID4, Elo: 835},

		{Mode: model.ModeCorrespondence1, ID: userID3, Elo: 900},
		{Mode: model.ModeCorrespondence1, ID: userID4, Elo: 2000},

		{Mode: model.ModeTimed15Plus10, ID: userID1, Elo: 1030},

		{Mode: model.ModeTimed3Plus2, ID: userID1, Elo: 1020},
		{Mode: model.ModeTimed3Plus2, ID: userID2, Elo: 900},

		{Mode: model.ModeTimed1Plus0, ID: userID1, Elo: 1050},
		{Mode: model.ModeTimed1Plus0, ID: userID4, Elo: 1010},
	} {
		modeLbZSet := cache.FmtLeaderboardZSet(change.Mode.String())

		err := testinfra.Redis.PrimaryClient.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.Elo, Member: change.ID}).Err()
		require.NoError(t, err)
	}

	persona1, err := service.GetPersona(ctx, userID1, 0)
	require.NoError(t, err)

	persona2, err := service.GetPersona(ctx, userID2, 0)
	require.NoError(t, err)

	persona3, err := service.GetPersona(ctx, userID3, 0)
	require.NoError(t, err)

	persona4, err := service.GetPersona(ctx, userID4, 0)
	require.NoError(t, err)

	wantModeStats := map[int64][]model.ModeStats{
		userID1: {
			// userID1 has played all the following games,
			{Mode: model.ModeCorrespondence7, Rank: 1, Wins: 2, Losses: 2, Draws: 0, Winrate: 50, Elo: 1000, HighestElo: 1000, Score: 1000},
			{Mode: model.ModeTimed3Plus2, Rank: 1, Wins: 5, Losses: 4, Draws: 0, Winrate: 55, Elo: 1020, HighestElo: 1020, Score: 1020},
			{Mode: model.ModeTimed15Plus10, Rank: 1, Wins: 4, Losses: 3, Draws: 0, Winrate: 57, Elo: 1030, HighestElo: 1030, Score: 1030},
			{Mode: model.ModeTimed1Plus0, Rank: 1, Wins: 6, Losses: 5, Draws: 0, Winrate: 54, Elo: 1050, HighestElo: 1050, Score: 1050},
		},
		userID2: {
			// userID2 has only played Timed1Plus0 - it is missing from the index (edge case) return default values
			{Mode: model.ModeTimed1Plus0, Rank: 0, Wins: 0, Losses: 0, Draws: 0, Winrate: 0, Elo: 1000, HighestElo: 1000},
		},
		userID3: {
			// userID3 is in 2nd place for Correspondence7: userID1 and userID2 have not played
			{Mode: model.ModeCorrespondence1, Rank: 2, Wins: 0, Losses: 0, Draws: 0, Winrate: 0, Elo: 900, HighestElo: 900, Score: 900},
			// userID2 is in 2nd place for Correspondence7: all users have stats for Correspondence7
			{Mode: model.ModeCorrespondence7, Rank: 2, Wins: 0, Losses: 0, Draws: 0, Winrate: 0, Elo: 900, HighestElo: 900, Score: 900},
		},
		userID4: {
			// userID4 is in 1st place for Correspondence1 (the only mode it has played)
			{Mode: model.ModeCorrespondence1, Rank: 1, Wins: 0, Losses: 0, Draws: 0, Winrate: 0, Elo: 2000, HighestElo: 2000, Score: 2000},
		},
	}

	testutil.Equal(t, wantModeStats[userID1], persona1.Stats.ModeStats)
	testutil.Equal(t, wantModeStats[userID2], persona2.Stats.ModeStats)
	testutil.Equal(t, wantModeStats[userID3], persona3.Stats.ModeStats)
	testutil.Equal(t, wantModeStats[userID4], persona4.Stats.ModeStats)
}
