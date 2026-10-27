package service

import (
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchUsers_Database(t *testing.T) {
	testinfra := itest.SetupIntegrationTest(t)
	defer testinfra.Close()

	itest.SetupElasticSearchPrecondition(t, testinfra.Es)

	service := NewUserSearchService(testinfra.Database.Querier, nil)

	users, err := service.SearchUsers(t.Context(), "john", 1, 20)
	require.NoError(t, err)

	wantUsers := []model.LbdUser{
		{
			ID:         8,
			Username:   "john",
			Country:    "us",
			Bio:        "",
			JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Elo:        1500,
			HighestElo: 2000,
			Wins:       12,
			Losses:     4,
			Winrate:    66,
			Rank:       1,
		},
		{
			ID:         9,
			Username:   "johnny",
			Country:    "us",
			Bio:        "",
			JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
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

func TestSearchUsers_ElasticSearch(t *testing.T) {
	testinfra := itest.SetupIntegrationTest(t)
	defer testinfra.Close()

	service := NewUserSearchService(testinfra.Database.Querier, testinfra.Es)

	service.FeatureFlags.IsUserSearchElasticSearch = true

	documents := []map[string]any{
		{"id": "INVALID", "username": "john", "userBiography": "matching bio", "createdAt": time.Now()},
		{"id": "9", "username": "johnny", "userBiography": "A matching bio", "createdAt": time.Now()},
	}
	for _, doc := range documents {
		result, err := testinfra.Es.Index(cloud.UserEsIndex).
			Id(doc["id"].(string)).
			Request(doc).
			Refresh(refresh.True).
			Do(t.Context())
		require.NoError(t, err)

		t.Logf("inserted into elastic search index: %+v", result)
	}

	events := []model.UpdateUserEvent{
		{ID: 8},
	}
	for _, event := range events {
		err := service.UpdateUserSearch(t.Context(), event)
		require.NoError(t, err)
	}

	users, err := service.SearchUsers(t.Context(), "john matching bio", 1, 20)
	require.NoError(t, err)

	wantUsers := []model.LbdUser{
		{
			ID:         8,
			Username:   "john",
			Country:    "us",
			Bio:        "",
			JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Elo:        1500,
			HighestElo: 2000,
			Wins:       12,
			Losses:     4,
			Winrate:    66,
			Rank:       1,
		},
		{
			ID:         9,
			Username:   "johnny",
			Country:    "us",
			Bio:        "",
			JoinedOn:   time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
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
