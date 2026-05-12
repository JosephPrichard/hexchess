package svc

import (
	"context"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/internal/enum"
	"hexchess-svc/internal/testutil"
	"hexchess-svc/model"

	"testing"
	"time"

	"hexchess-svc/internal/logutil"
	"hexchess-svc/itest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReplay(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	t.Run("GetReplay", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		actualReplay1, err := services.GetReplay(ctx, itest.FirstReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplays[0], actualReplay1)
	})

	t.Run("GetReplayWithGuest", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		actualReplay1, err := services.GetReplay(ctx, itest.GuestReplayID)
		require.NoError(t, err)

		assert.Equal(t, itest.TestReplays[2], actualReplay1)
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
				UserID:  enum.Just(int64(1)),
				AfterID: enum.Nothing[int64](),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{itest.TestReplays[2], itest.TestReplays[1], itest.TestReplays[0]},
		},
		{
			name: "QueryBy_Users_Cursor",
			replayQuery: ReplaysQuery{
				UserID:  enum.Just(int64(1)),
				AfterID: enum.Just(int64(3)),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{itest.TestReplays[0]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, _ := setupServicesTest(t, serviceMocks{}, itest.ROPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

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

			services, _ := setupServicesTest(t, serviceMocks{}, itest.ROPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			eloHistories, bd, err := services.RetrieveEloHistoryBuckets(ctx, test.params)
			require.NoError(t, err)

			testutil.Equal(t, test.wantEloBuckets, eloHistories, cmpopts.IgnoreFields(EloHistoryBucket{}, "Timestamp"))
			testutil.Equal(t, test.wantBucketDuration, bd)
		})
	}
}
