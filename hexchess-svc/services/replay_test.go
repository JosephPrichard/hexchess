package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

func TestInsertThenGetReplay(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-insert-get")

	// when
	id, err := InsertReplay(ctx, pdb.Query, ReplayInst{
		WhiteID:            2,
		BlackID:            3,
		Result:             WhiteWin,
		Cause:              Checkmate,
		Mode:               ModeCorrespondence7,
		WinEloDiff:         35,
		LoseEloDiff:        -25,
		ReplayWhiteElo:     1050,
		ReplayBlackElo:     950,
		PlayedOn:           TestTimeNow,
		SerializedMoveHist: []byte{},
	})
	require.NoError(t, err)

	actualReplay1, err := GetReplay(ctx, pdb.Query, id)
	require.NoError(t, err)

	// then
	wantReplay := ReplayEntity{
		ID:           id,
		WhiteID:      2,
		BlackID:      3,
		WhiteName:    "user2",
		BlackName:    "user3",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         ModeCorrespondence7,
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinEloDiff:   35,
		LoseEloDiff:  -25,
		WhiteEloDiff: 35,
		BlackEloDiff: -25,
		WhiteElo:     1000,
		BlackElo:     900,
		PlayedOn:     TestTimeNow.Local(),
	}
	assert.Equal(t, wantReplay, actualReplay1)
}

func TestGetUserReplays(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-get-replays")

	// when
	actualReplayList1, err := GetUserReplays(ctx, pdb.Query, 1, -1, 5)
	require.NoError(t, err)
	actualReplayList2, err := GetUserReplays(ctx, pdb.Query, 1, 3, 5)
	require.NoError(t, err)

	// then
	replay1 := TestReplayEntities[1]
	replay3 := TestReplayEntities[0]
	expectedReplayList1 := []ReplayEntity{replay3, replay1}
	expectedReplayList2 := []ReplayEntity{replay1}

	assert.Equal(t, expectedReplayList1, actualReplayList1)
	assert.Equal(t, expectedReplayList2, actualReplayList2)
}

func TestGetReplayMoveList(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-get-move-list")

	// when and then
	_, err := GetReplayMoveHistory(ctx, pdb.Query, 1)
	require.NoError(t, err)
}

func TestRetrieveEloHistories(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-retrieve-elo-histories")

	timeUntil := time.Date(2020, 2, 2, 2, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		params             EloHistoriesParams
		wantBucketDuration time.Duration
		wantEloBuckets     EloHistoryBuckets
	}{
		{
			params:             EloHistoriesParams{UserID: 6, TimeUntil: timeUntil},
			wantBucketDuration: LongBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				ModeCorrespondence7: []EloHistoryBucket{
					{Timestamp: "1899-12-31T18:00:00-06:00", Elo: 1030},
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1090},
				},
				ModeCorrespondence1: []EloHistoryBucket{
					{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1030},
				},
			},
		},
		{
			params:             EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			wantBucketDuration: ShortBucketDuration,
			wantEloBuckets: EloHistoryBuckets{
				ModeCorrespondence7: []EloHistoryBucket{
					{Timestamp: "2019-12-31T18:00:00-06:00", Elo: 1075},
					{Timestamp: "2020-01-02T18:00:00-06:00", Elo: 1120},
				},
				ModeCorrespondence1: []EloHistoryBucket{
					{Timestamp: "2020-01-04T18:00:00-06:00", Elo: 1030},
				},
			},
		},
	} {
		eloHistories, bd, err := RetrieveEloHistoryBuckets(ctx, &dbs, test.params)
		require.NoError(t, err)
		assert.Equal(t, test.wantEloBuckets, eloHistories)
		assert.Equal(t, test.wantBucketDuration, bd)
	}
}
