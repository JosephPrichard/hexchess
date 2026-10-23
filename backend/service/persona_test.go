package service

import (
	"context"
	"errors"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPersonaServices(t slogutil.TestLogger) (*PersonaService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewPersonaService(
		NewUserService(infra.Database),
		NewLeaderboardService(infra.Redis, infra.Database.Querier()),
		NewReplaySearchService(infra.Database),
	)

	return service, infra
}

func TestGetPersona(t *testing.T) {
	service, testinfra := setupPersonaServices(t)
	defer testinfra.Close()

	persona, err := service.GetPersona(t.Context(), 1, 10)
	require.NoError(t, err)

	wantPersona := model.Persona{
		User:  itest.TestUser[0],
		Stats: itest.TestUserStats[0],
		ReplayList: []model.FullReplay{
			itest.TestReplays[4],
			itest.TestReplays[3],
			itest.TestReplays[2],
			itest.TestReplays[0],
		},
	}

	testutil.Equal(t, persona, wantPersona)
}

type failingUserGetter struct {
	getByIDErr      error
	getUserStatsErr error
}

func (querier *failingUserGetter) GetUserByID(_ context.Context, _ int64) (model.User, error) {
	return model.User{}, querier.getByIDErr
}

func (querier *failingUserGetter) GetUserStats(_ context.Context, _ int64) (model.UserStats, error) {
	return model.UserStats{}, querier.getUserStatsErr
}

type failingLeaderboardGetter struct {
	err error
}

func (querier *failingLeaderboardGetter) GetUserLeaderboardRanks(_ context.Context, _ int64, _ map[string]model.GameMode) (map[string]LeaderboardRank, error) {
	return nil, querier.err
}

type failingReplayQuerier struct {
	err error
}

func (querier *failingReplayQuerier) SearchReplaysByQuery(_ context.Context, _ ReplaysQuery) ([]model.FullReplay, error) {
	return nil, querier.err
}

func TestGetPersona_PropagatesFailures(t *testing.T) {
	testinfra := itest.SetupIntegrationTest(t)
	defer testinfra.Close()

	tests := []struct {
		name        string
		user        PersonaUserGetter
		replay      ReplayQuerier
		leaderboard LeaderboardGetter
		wantError   error
	}{
		{
			name:        "FailingGetUserByID",
			user:        &failingUserGetter{getByIDErr: errors.New("failed get user by id call")},
			leaderboard: NewLeaderboardService(testinfra.Redis, testinfra.Database.Querier()),
			replay:      NewReplaySearchService(testinfra.Database),
			wantError:   errors.New("failed get user by id call"),
		},
		{
			name:        "FailingGetUserStats",
			user:        &failingUserGetter{getUserStatsErr: errors.New("failed get user stats call")},
			leaderboard: NewLeaderboardService(testinfra.Redis, testinfra.Database.Querier()),
			replay:      NewReplaySearchService(testinfra.Database),
			wantError:   errors.New("failed get user stats call"),
		},
		{
			name:        "FailingReplayQuerier",
			user:        NewUserService(testinfra.Database),
			leaderboard: NewLeaderboardService(testinfra.Redis, testinfra.Database.Querier()),
			replay:      &failingReplayQuerier{err: errors.New("failed replay call")},
			wantError:   errors.New("failed replay call"),
		},
		{
			name:        "FailingLeaderboardGetter",
			user:        NewUserService(testinfra.Database),
			leaderboard: &failingLeaderboardGetter{err: errors.New("failed leaderboard call")},
			replay:      NewReplaySearchService(testinfra.Database),
			wantError:   errors.New("failed leaderboard call"),
		},
		{
			name:        "FailingAllCalls",
			user:        &failingUserGetter{getByIDErr: errors.New("failed get user by id call"), getUserStatsErr: errors.New("failed get user stats call")},
			leaderboard: &failingLeaderboardGetter{err: errors.New("failed leaderboard call")},
			replay:      &failingReplayQuerier{err: errors.New("failed replay call")},
			wantError:   errors.New("failed get user by id call"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewPersonaService(test.user, test.leaderboard, test.replay)

			_, err := service.GetPersona(t.Context(), 1, 10)
			leafErr := leafError(err)

			assert.Equal(t, test.wantError, leafErr)
		})
	}
}
