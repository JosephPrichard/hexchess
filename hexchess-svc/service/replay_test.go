package svc

import (
	"context"
	"hexchess-svc/model"

	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReplay(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	t.Run("GetReplay", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		actualReplay1, err := services.GetReplay(ctx, itest.FirstReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplay[0], actualReplay1)
	})

	t.Run("GetReplayWithGuest", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		actualReplay1, err := services.GetReplay(ctx, itest.GuestReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplay[2], actualReplay1)
	})
}

func TestGetUserReplays(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	actualReplayList1, err := services.GetUserReplays(ctx, 1, -1, 5)
	require.NoError(t, err)
	actualReplayList2, err := services.GetUserReplays(ctx, 1, 3, 5)
	require.NoError(t, err)

	replay1 := itest.TestReplay[0]
	replay3 := itest.TestReplay[1]
	replay4 := itest.TestReplay[2]
	expectedReplayList1 := []model.FullReplay{replay4, replay3, replay1}
	expectedReplayList2 := []model.FullReplay{replay1}

	assert.Equal(t, expectedReplayList1, actualReplayList1)
	assert.Equal(t, expectedReplayList2, actualReplayList2)
}

func TestRetrieveEloHistories(t *testing.T) {
	t.Parallel()

	timeUntil := time.Date(2020, 2, 2, 2, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		name               string
		params             EloHistoriesParams
		wantBucketDuration time.Duration
		wantEloBuckets     EloHistoryBuckets
	}{
		{
			name:               "retrieve all elo histories",
			params:             EloHistoriesParams{UserID: 6, TimeUntil: timeUntil},
			wantBucketDuration: LongBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				model.ModeCorrespondence7.String(): []EloHistoryBucket{
					{Timestamp: "1899-12-31T18:00:00-06:00", Elo: 1030},
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1090},
				},
				model.ModeCorrespondence1.String(): []EloHistoryBucket{
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1030},
				},
			},
		},
		{
			name:               "retrieve elo histories past 3 months",
			params:             EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			wantBucketDuration: ShortBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				model.ModeCorrespondence7.String(): []EloHistoryBucket{
					{Timestamp: "2019-12-31T18:00:00-06:00", Elo: 1075},
					{Timestamp: "2020-01-02T18:00:00-06:00", Elo: 1120},
				},
				model.ModeCorrespondence1.String(): []EloHistoryBucket{
					{Timestamp: "2020-01-04T18:00:00-06:00", Elo: 1030},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {

			services, _ := SetupServicesTest(t, Mocks{}, itest.ROPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			eloHistories, bd, err := services.RetrieveEloHistoryBuckets(ctx, test.params)
			require.NoError(t, err)

			assert.Equal(t, test.wantEloBuckets, eloHistories)
			assert.Equal(t, test.wantBucketDuration, bd)
		})
	}
}
