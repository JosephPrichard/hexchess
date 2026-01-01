package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReplay(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Postgres: pdb}

	// when
	actualReplay1, err := s.GetReplay(ctx, 1)
	require.NoError(t, err)

	// then
	wantReplay := ReplayEntity{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         "CORRESPONDENCE_7",
		Result:       "WHITE_WINS",
		Cause:        "CHECKMATE",
		WinEloDiff:   30,
		LoseEloDiff:  -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
		PlayedOn:     db.TestTimeNow.Local(),
	}
	assert.Equal(t, wantReplay, actualReplay1)
}

func TestGetUserReplays(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Postgres: pdb}

	// when
	actualReplayList1, err := s.GetUserReplays(ctx, 1, -1, 5)
	require.NoError(t, err)
	actualReplayList2, err := s.GetUserReplays(ctx, 1, 3, 5)
	require.NoError(t, err)

	// then
	replay1 := TestReplayEntities[0]
	replay3 := TestReplayEntities[1]
	expectedReplayList1 := []ReplayEntity{replay3, replay1}
	expectedReplayList2 := []ReplayEntity{replay1}

	assert.Equal(t, expectedReplayList1, actualReplayList1)
	assert.Equal(t, expectedReplayList2, actualReplayList2)
}

func TestGetReplayMoveList(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Postgres: pdb}

	// when and then
	_, err := s.GetReplayMoveHistory(ctx, 1)
	require.NoError(t, err)
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
				ModeCorrespondence7.String(): []EloHistoryBucket{
					{Timestamp: "1899-12-31T18:00:00-06:00", Elo: 1030},
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1090},
				},
				ModeCorrespondence1.String(): []EloHistoryBucket{
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1030},
				},
			},
		},
		{
			name:               "retrieve elo histories past 3 months",
			params:             EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			wantBucketDuration: ShortBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				ModeCorrespondence7.String(): []EloHistoryBucket{
					{Timestamp: "2019-12-31T18:00:00-06:00", Elo: 1075},
					{Timestamp: "2020-01-02T18:00:00-06:00", Elo: 1120},
				},
				ModeCorrespondence1.String(): []EloHistoryBucket{
					{Timestamp: "2020-01-04T18:00:00-06:00", Elo: 1030},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			pdb, closer := db.BeforePostgresTest(t, false)
			defer closer()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)
			s := State{Postgres: pdb}

			// when
			eloHistories, bd, err := s.RetrieveEloHistoryBuckets(ctx, test.params)
			require.NoError(t, err)

			// then
			assert.Equal(t, test.wantEloBuckets, eloHistories)
			assert.Equal(t, test.wantBucketDuration, bd)
		})
	}
}
