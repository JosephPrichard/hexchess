package svc

import (
	"context"
	"hexchess-svc/lib/optional"
	"hexchess-svc/lib/testutil"
	"hexchess-svc/model"

	"github.com/google/go-cmp/cmp/cmpopts"

	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/lib/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReplay(t *testing.T) {
	t.Parallel()

	services, testinfra := setupServicesTest(t, nil, itest.RWPostgres)
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

func TestSearchReplaysByQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		replayQuery ReplaysQuery
		wantReplays []model.FullReplay
	}{
		{
			name: "QueryBy_Users",
			replayQuery: ReplaysQuery{
				UserID:  optional.Just(int64(1)),
				AfterID: optional.Nothing[int64](),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[2],
				itest.TestReplays[0],
			},
		},
		{
			name: "QueryBy_Users_Cursor",
			replayQuery: ReplaysQuery{
				UserID:  optional.Just(int64(1)),
				AfterID: optional.Just(int64(3)),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{itest.TestReplays[0]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupServicesTest(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			ctx := t.Context()

			replayList, err := services.SearchReplaysByQuery(ctx, tt.replayQuery)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantReplays, replayList)
		})
	}
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
					{Elo: 1030},
					{Elo: 1090},
				},
				model.ModeCorrespondence1.String(): []EloHistoryBucket{
					{Elo: 1030},
				},
			},
		},
		{
			name:               "retrieve elo histories past 3 months",
			params:             EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			wantBucketDuration: ShortBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				model.ModeCorrespondence7.String(): []EloHistoryBucket{
					{Elo: 1075},
					{Elo: 1120},
				},
				model.ModeCorrespondence1.String(): []EloHistoryBucket{
					{Elo: 1030},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {

			services, testinfra := setupServicesTest(t, nil, itest.ROPostgres)
			defer testinfra.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			resp, err := services.RetrieveEloHistoryBuckets(ctx, test.params)
			require.NoError(t, err)

			testutil.Equal(t, test.wantEloBuckets, resp.EloHistories, cmpopts.IgnoreFields(EloHistoryBucket{}, "Timestamp"))
			testutil.Equal(t, test.wantBucketDuration, resp.BucketDuration)
		})
	}
}
