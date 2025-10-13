package dal

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSetThenGetState(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	id1 := "test-id1-" + uuid.NewString()
	id2 := "test-id2-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	ctx := context.WithValue(context.Background(), util.TraceKey, "test-set-then-get")

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

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-set-then-get-views")

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

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-set-then-get-user")

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
