package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/util"
	"testing"
)

func TestInsertThenGet(t *testing.T) {
	pgDB, closer := BeforeDbTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-insert-get")

	id, err := InsertReplay(ctx, pgDB.Q, ReplayInst{2, 3, WhiteWin, Checkmate, 35, -25, []byte{}})
	assert.NoError(t, err)

	actualReplay1, err := GetReplay(ctx, pgDB.Q, id)
	assert.NoError(t, err)

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
		WinElo:       35,
		LoseElo:      -25,
		WhiteEloDiff: 35,
		BlackEloDiff: -25,
		WhiteElo:     1000,
		BlackElo:     900,
	}
	util.AssertEqualIgnoring(t, expReplay, actualReplay1, ReplayEntityCmpOpts)
}

func TestGetUserReplays(t *testing.T) {
	pgDB, closer := BeforeDbTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-get-replays")

	actualReplayList1, err := GetUserReplays(ctx, pgDB.Q, 1, -1, 5)
	assert.NoError(t, err)
	actualReplayList2, err := GetUserReplays(ctx, pgDB.Q, 1, 3, 5)
	assert.NoError(t, err)

	replay1 := ReplayEntity{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinElo:       30,
		LoseElo:      -30,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
		WhiteElo:     1000,
		BlackElo:     1000,
	}
	replay3 := ReplayEntity{
		ID:           3,
		WhiteID:      3,
		BlackID:      1,
		WhiteName:    "user3",
		BlackName:    "user1",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       Draw,
		Cause:        Checkmate,
		WinElo:       30,
		LoseElo:      -30,
		WhiteElo:     900,
		BlackElo:     1000,
	}

	expectedReplayList1 := []ReplayEntity{replay3, replay1}
	expectedReplayList2 := []ReplayEntity{replay1}

	util.AssertEqualIgnoring(t, expectedReplayList1, actualReplayList1, ReplayEntityCmpOpts)
	util.AssertEqualIgnoring(t, expectedReplayList2, actualReplayList2, ReplayEntityCmpOpts)
}

func TestGetReplayMoveList(t *testing.T) {
	pgDB, closer := BeforeDbTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-get-move-list")

	_, err := GetReplayMoveHistory(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
}
