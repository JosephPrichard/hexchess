package service

import (
	"context"
	"hexchess-svc/utils/testutil"

	"github.com/google/go-cmp/cmp/cmpopts"

	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/utils/slogutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupReplayTest(t slogutil.TestLogger) (*ReplayService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewReplayService(infra.Database)

	return services, infra
}

func TestGetReplay(t *testing.T) {
	services, testinfra := setupReplayTest(t)
	defer testinfra.Close()

	t.Run("GetReplay", func(t *testing.T) {
		ctx := t.Context()

		actualReplay1, err := services.GetReplay(ctx, itest.FirstReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplays[0], actualReplay1)
	})

	t.Run("GetReplayWithGuest", func(t *testing.T) {
		ctx := t.Context()

		actualReplay1, err := services.GetReplay(ctx, itest.GuestReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplays[3], actualReplay1)
	})
}

func TestRetrieveEloHistories(t *testing.T) {
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
				"CORRESPONDENCE_7": []EloHistoryBucket{
					{Elo: 1030},
					{Elo: 1090},
				},
				"CORRESPONDENCE_1": []EloHistoryBucket{
					{Elo: 1030},
				},
			},
		},
		{
			name:               "retrieve elo histories past 3 months",
			params:             EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			wantBucketDuration: ShortBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				"CORRESPONDENCE_7": []EloHistoryBucket{
					{Elo: 1075},
					{Elo: 1120},
				},
				"CORRESPONDENCE_1": []EloHistoryBucket{
					{Elo: 1030},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {

			services, testinfra := setupReplayTest(t)
			defer testinfra.Close()

			ctx := context.WithValue(t.Context(), slogutil.Trace, test.name)

			resp, err := services.RetrieveEloHistoryBuckets(ctx, test.params)
			require.NoError(t, err)

			testutil.Equal(t, test.wantEloBuckets, resp.EloHistories, cmpopts.IgnoreFields(EloHistoryBucket{}, "Timestamp"))
			testutil.Equal(t, test.wantBucketDuration, resp.BucketDuration)
		})
	}
}
