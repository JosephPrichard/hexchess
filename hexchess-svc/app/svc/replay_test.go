package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	Replay1 = ReplayEntity{
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

	Replay2 = ReplayEntity{
		ID:           2,
		WhiteID:      2,
		BlackID:      3,
		WhiteName:    "user2",
		BlackName:    "user3",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       BlackWin,
		Cause:        Checkmate,
		WinElo:       30,
		LoseElo:      -30,
		WhiteElo:     1000,
		BlackElo:     900,
	}

	Replay3 = ReplayEntity{
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
)

func TestInsertThenGet(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-insert-get")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "{}"}))
	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{2, 3, int32(BlackWin), int32(Checkmate), 30, -30, "{}"}))
	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{3, 1, int32(Draw), int32(Checkmate), 30, -30, "{}"}))

	actualReplay1, err := GetReplay(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	actualReplay2, err := GetReplay(ctx, pgDB.Q, 2)
	assert.NoError(t, err)
	actualReplay3, err := GetReplay(ctx, pgDB.Q, 3)
	assert.NoError(t, err)

	assert.Equal(t, Replay1, actualReplay1)
	assert.Equal(t, Replay2, actualReplay2)
	assert.Equal(t, Replay3, actualReplay3)
}

func TestGetUserReplays(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-get-replays")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "{}"}))
	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{2, 3, int32(BlackWin), int32(Checkmate), 30, -30, "{}"}))
	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{3, 1, int32(Draw), int32(Checkmate), 30, -30, "{}"}))

	actualReplayList1, err := GetUserReplays(ctx, pgDB.Q, 1, -1, 5)
	assert.NoError(t, err)
	actualReplayList2, err := GetUserReplays(ctx, pgDB.Q, 1, 3, 5)
	assert.NoError(t, err)

	expectedReplayList1 := []ReplayEntity{Replay3, Replay1}
	expectedReplayList2 := []ReplayEntity{Replay1}

	assert.Equal(t, expectedReplayList1, actualReplayList1)
	assert.Equal(t, expectedReplayList2, actualReplayList2)
}

func TestGetReplayMoveList(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-get-move-list")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, InsertReplay(ctx, pgDB.Q, ReplayInst{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "[]"}))
	actualMoveList, err := GetReplayMoveList(ctx, pgDB.Q, 1)
	assert.NoError(t, err)

	assert.Equal(t, "[]", actualMoveList)
}
