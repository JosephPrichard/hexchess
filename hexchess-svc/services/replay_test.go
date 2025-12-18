package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestInsertThenGetReplay(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-insert-get")

	// when
	id, err := InsertReplay(ctx, pdb.Query, ReplayInst{
		WhiteID:            2,
		BlackID:            3,
		Result:             WhiteWin,
		Cause:              Checkmate,
		Mode:               ModeUnlimited,
		WinEloDiff:         35,
		LoseEloDiff:        -25,
		ReplayWhiteElo:     1050,
		ReplayBlackElo:     950,
		PlayedOn:           TestTimeNow,
		SerializedMoveHist: []byte{},
	})
	assert.NoError(t, err)

	actualReplay1, err := GetReplay(ctx, pdb.Query, id)
	assert.NoError(t, err)

	// then
	expReplay := ReplayEntity{
		ID:           id,
		WhiteID:      2,
		BlackID:      3,
		WhiteName:    "user2",
		BlackName:    "user3",
		WhiteCountry: "us",
		BlackCountry: "us",
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
	assert.Equal(t, expReplay, actualReplay1)
}

func TestGetUserReplays(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-get-replays")

	// when
	actualReplayList1, err := GetUserReplays(ctx, pdb.Query, 1, -1, 5)
	assert.NoError(t, err)
	actualReplayList2, err := GetUserReplays(ctx, pdb.Query, 1, 3, 5)
	assert.NoError(t, err)

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

	ctx := context.WithValue(t.Context(), util.Trace, "testing-get-move-list")

	// when and then
	_, err := GetReplayMoveHistory(ctx, pdb.Query, 1)
	assert.NoError(t, err)
}

func TestRetrieveEloHistories(t *testing.T) {
	dbs, closer := db.BeforeDbTest(t, false, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-retrieve-elo-histories")

	timeUntil := time.Date(2020, 2, 2, 2, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		params            EloHistoriesParams
		expBucketDuration time.Duration
		expEloBuckets     EloHistoryBuckets
	}{
		{
			params:            EloHistoriesParams{UserID: 6, TimeUntil: timeUntil},
			expBucketDuration: LongBucketDuration,
			expEloBuckets: EloHistoryBuckets{
				{Timestamp: "1899-12-31T18:00:00-06:00", Elo: 1030},
				{Timestamp: "2019-12-29T18:00:00-06:00", Elo: 1090},
			},
		},
		{
			params:            EloHistoriesParams{UserID: 6, Months: 3, TimeUntil: timeUntil},
			expBucketDuration: ShortBucketDuration,
			expEloBuckets: EloHistoryBuckets{
				{Timestamp: "2019-12-31T18:00:00-06:00", Elo: 1075},
				{Timestamp: "2020-01-02T18:00:00-06:00", Elo: 1120},
			},
		},
	} {
		eloHistories, bd, err := RetrieveEloHistoryBuckets(ctx, &dbs, test.params)
		assert.NoError(t, err)
		assert.Equal(t, test.expEloBuckets, eloHistories)
		assert.Equal(t, test.expBucketDuration, bd)
	}
}
