package svc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestSetThenGetState(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	id1 := "test-id1-" + uuid.NewString()
	id2 := "test-id2-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	ctx := context.WithValue(context.Background(), TraceKey, "test-set-then-get")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)

	outState, err := GetChessState(ctx, rdb, id1)
	assert.NoError(t, err)

	state1.Touch = time.Time{}
	outState.Touch = time.Time{}
	assert.Equal(t, state1, outState)

	_, err = GetChessState(ctx, rdb, id2)
	assert.Error(t, ErrNoChessState, err)
}

func TestSetThenGetViews(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	id1 := "test-id1-" + uuid.NewString()
	id2 := "test-id2-" + uuid.NewString()
	id3 := "test-id3-" + uuid.NewString()
	id4 := "test-id4-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	state2 := MakeStartChessState(id2, RealTime)
	state3 := MakeStartChessState(id3, RealTime)
	state4 := MakeStartChessState(id4, RealTime)

	ctx := context.WithValue(context.Background(), TraceKey, "test-set-then-get-views")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id2, state2)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id3, state3)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id4, state4)
	assert.NoError(t, err)

	viewsList1, err := GetAllChessViews(ctx, rdb, 1, 2)
	assert.NoError(t, err)
	viewsList2, err := GetAllChessViews(ctx, rdb, 2, 2)
	assert.NoError(t, err)

	expectedViewList1 := []ChessView{
		{ID: id4, FirstColor: Random, TimeControl: RealTime},
		{ID: id3, FirstColor: Random, TimeControl: RealTime},
	}
	expectedViewList2 := []ChessView{
		{ID: id2, FirstColor: Random, TimeControl: RealTime},
		{ID: id1, FirstColor: Random, TimeControl: RealTime},
	}

	assert.Equal(t, expectedViewList1, viewsList1)
	assert.Equal(t, expectedViewList2, viewsList2)
}

func TestSetThenGetUserViews(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	id1 := "test-id1-" + uuid.NewString()
	id2 := "test-id2-" + uuid.NewString()
	id3 := "test-id3-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	state2 := MakeStartChessState(id2, RealTime)
	state3 := MakeStartChessState(id3, RealTime)

	state1.WhitePlayer = &PlayerState{ID: 1}
	state1.BlackPlayer = &PlayerState{ID: 2}
	state2.BlackPlayer = &PlayerState{ID: 1}

	ctx := context.WithValue(context.Background(), TraceKey, "test-set-then-get-user")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id2, state2)
	assert.NoError(t, err)
	_, err = SetChessState(ctx, rdb, id3, state3)
	assert.NoError(t, err)

	viewsList1, err := GetUserChessViews(ctx, rdb, 1)
	assert.NoError(t, err)
	viewsList2, err := GetUserChessViews(ctx, rdb, 2)
	assert.NoError(t, err)
	viewsList3, err := GetUserChessViews(ctx, rdb, 3)
	assert.NoError(t, err)

	expectedViewList1 := []ChessView{
		{ID: id2, BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime},
		{ID: id1, WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime},
	}
	expectedViewList2 := []ChessView{
		{ID: id1, WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime},
	}

	assert.Equal(t, expectedViewList1, viewsList1)
	assert.Equal(t, expectedViewList2, viewsList2)
	assert.Empty(t, viewsList3)
}

func TestSessions(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	player1 := PlayerState{ID: 1, Name: "test-name1"}
	sessionID := "session1" + uuid.NewString()

	ctx := context.WithValue(context.Background(), TraceKey, "test-sessions")

	err := SetSession(ctx, rdb, sessionID, player1, 100*time.Second)
	assert.NoError(t, err)

	player3, err := GetSession(ctx, rdb, sessionID)
	assert.NoError(t, err)

	assert.Equal(t, player1, player3)
}

func TestLeaderboard(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	id1 := int64(rand.Intn(math.MaxInt64))
	id2 := int64(rand.Intn(math.MaxInt64))
	id3 := int64(rand.Intn(math.MaxInt64))
	id4 := int64(rand.Intn(math.MaxInt64))

	ctx := context.WithValue(context.Background(), TraceKey, "test-leaderboard")

	assert.NoError(t, IncrLeaderboard(ctx, rdb, IncrLbChangeSet{id1, 1500}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, IncrLbChangeSet{id2, 1000}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, IncrLbChangeSet{id3, 950}))
	assert.NoError(t, IncrLeaderboard(ctx, rdb, IncrLbChangeSet{id4, 835}))

	rank1, err := GetLeaderboardRank(ctx, rdb, id1)
	assert.NoError(t, err)
	rank2, err := GetLeaderboardRank(ctx, rdb, id2)
	assert.NoError(t, err)
	rank3, err := GetLeaderboardRank(ctx, rdb, id3)
	assert.NoError(t, err)
	rank4, err := GetLeaderboardRank(ctx, rdb, id4)
	assert.NoError(t, err)

	assert.Equal(t, 1, rank1)
	assert.Equal(t, 2, rank2)
	assert.Equal(t, 3, rank3)
	assert.Equal(t, 4, rank4)

	leaderboard1, err := GetLeaderboard(ctx, rdb, 0, 4)
	assert.NoError(t, err)

	assert.NoError(t, IncrLeaderboard(ctx, rdb, IncrLbChangeSet{id2, 30}))

	leaderboard2, err := GetLeaderboard(ctx, rdb, 1, 2)
	assert.NoError(t, err)

	expectedLeaderboard1 := Leaderboard{
		Users:     []RankedUser{{ID: id1, Rank: 1}, {ID: id2, Rank: 2}, {ID: id3, Rank: 3}, {ID: id4, Rank: 4}},
		PageCount: 1,
	}

	expectedLeaderboard2 := Leaderboard{
		Users:     []RankedUser{{ID: id2, Rank: 2}, {ID: id3, Rank: 3}},
		PageCount: 2,
	}

	assert.Equal(t, expectedLeaderboard1, leaderboard1)
	assert.Equal(t, expectedLeaderboard2, leaderboard2)
}
