package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/logs"
	"testing"
	"time"
)

func TestSetThenGetState(t *testing.T) {
	rdb, closer := BeforeRedisTests(t)
	defer closer()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-set-then-get")

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

func TestSetThenGetUserViews(t *testing.T) {
	rdb, closer := BeforeRedisTests(t)
	defer closer()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	state1 := MakeStartChessState(id1, RealTime)
	state2 := MakeStartChessState(id2, RealTime)
	state3 := MakeStartChessState(id3, RealTime)

	state1.WhitePlayer = &PlayerState{ID: 1}
	state1.BlackPlayer = &PlayerState{ID: 2}
	state2.BlackPlayer = &PlayerState{ID: 1}
	state3.BlackPlayer = &PlayerState{ID: 1}

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-set-then-get-user")
	now := time.Now()

	_, err := SetChessStateAt(ctx, rdb, id1, state1, now.Add(-100))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id2, state2, now.Add(-50))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id3, state3, now.Add(-10))
	assert.NoError(t, err)

	viewsList1, err := GetUserChessViews(ctx, rdb, 1)
	assert.NoError(t, err)
	viewsList2, err := GetUserChessViews(ctx, rdb, 2)
	assert.NoError(t, err)
	viewsList3, err := GetUserChessViews(ctx, rdb, 3)
	assert.NoError(t, err)
	viewsList4, err := GetUserChessViewsPaged(ctx, rdb, 1, 1, 2)
	assert.NoError(t, err)
	viewsList5, err := GetUserChessViewsPaged(ctx, rdb, 1, 2, 2)
	assert.NoError(t, err)

	v1 := ChessView{ID: id1, WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime}
	v2 := ChessView{ID: id2, BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime}
	v3 := ChessView{ID: id3, BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime}

	assert.Equal(t, []ChessView{v3, v2, v1}, viewsList1)
	assert.Equal(t, []ChessView{v1}, viewsList2)
	assert.Empty(t, viewsList3)
	assert.Equal(t, []ChessView{v3, v2}, viewsList4)
	assert.Equal(t, []ChessView{v1}, viewsList5)
}
