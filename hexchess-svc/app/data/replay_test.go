package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/app/util"
	"testing"
)

var TestReplays = []ReplayInst{
	{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "[]"},
	{2, 3, int32(BlackWin), int32(Checkmate), 30, -30, "{}"},
	{3, 1, int32(Draw), int32(Checkmate), 30, -30, "{}"},
}

func createTestReplays(t *testing.T, pgDB DB, insts ...ReplayInst) {
	ctx := context.WithValue(context.Background(), util.TraceKey, "create-test-replays")
	for _, inst := range insts {
		_, err := InsertReplay(ctx, pgDB.Q, inst)
		if err != nil {
			t.Fatalf("failed to insert test replay: %v", err)
		}
	}
}

func TestInsertThenGet(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-insert-get")

	id, err := InsertReplay(ctx, pgDB.Q, ReplayInst{2, 3, int32(WhiteWin), int32(Checkmate), 35, -25, "{}"})
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
		WhiteElo:     1000,
		BlackElo:     900,
	}
	assert.Equal(t, expReplay, actualReplay1)
}

func TestGetUserReplays(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-get-replays")

	// the only replays that include userID '1' should be the replays made in the test init phase

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

	assert.Equal(t, expectedReplayList1, actualReplayList1)
	assert.Equal(t, expectedReplayList2, actualReplayList2)
}

func TestGetReplayMoveList(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-get-move-list")

	actualMoveList, err := GetReplayMoveList(ctx, pgDB.Q, 1)
	assert.NoError(t, err)

	assert.Equal(t, "[]", actualMoveList)
}
