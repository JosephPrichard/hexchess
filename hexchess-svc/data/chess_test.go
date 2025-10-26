package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/lib"
	"testing"
	"time"
)

func TestSetThenGetState(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()

	state1 := MakeState(id1, RealTime)
	ctx := context.WithValue(context.Background(), lib.TK, "testing-set-then-get")

	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)

	outState1, err := GetChessState(ctx, rdb, id1)
	assert.NoError(t, err)

	lib.AssertEqualIgnoring(t, state1, outState1, ChessMetaCmpOpts)

	_, err = GetChessState(ctx, rdb, id2)
	assert.Error(t, ErrNoChessState, err)
}

func TestSetThenGetUserViews(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	state1 := MakeState(id1, RealTime)
	state2 := MakeState(id2, RealTime)
	state3 := MakeState(id3, RealTime)

	state1.WhitePlayer = &PlayerState{ID: 1}
	state1.BlackPlayer = &PlayerState{ID: 2}
	state2.BlackPlayer = &PlayerState{ID: 1}
	state3.BlackPlayer = &PlayerState{ID: 1}

	ctx := context.WithValue(context.Background(), lib.TK, "testing-set-then-get-user")
	now := time.Now()

	_, err := SetChessStateAt(ctx, rdb, id1, state1, now.Add(-100))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id2, state2, now.Add(-50))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id3, state3, now.Add(-10))
	assert.NoError(t, err)

	metaList1, err := GetUserChessMetas(ctx, rdb, 1)
	assert.NoError(t, err)
	metaList2, err := GetUserChessMetas(ctx, rdb, 2)
	assert.NoError(t, err)
	metaList3, err := GetUserChessMetas(ctx, rdb, 3)
	assert.NoError(t, err)
	metaList4, err := GetUserChessViewsPaged(ctx, rdb, 1, 1, 2)
	assert.NoError(t, err)
	metaList5, err := GetUserChessViewsPaged(ctx, rdb, 1, 2, 2)
	assert.NoError(t, err)

	m1 := ChessMeta{ID: id1, WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: Random, TimeControl: RealTime}
	m2 := ChessMeta{ID: id2, BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime}
	m3 := ChessMeta{ID: id3, BlackPlayer: &PlayerState{ID: 1}, FirstColor: Random, TimeControl: RealTime}

	assert.Equal(t, []ChessMeta{m3, m2, m1}, metaList1)
	assert.Equal(t, []ChessMeta{m1}, metaList2)
	assert.Empty(t, metaList3)
	assert.Equal(t, []ChessMeta{m3, m2}, metaList4)
	assert.Equal(t, []ChessMeta{m1}, metaList5)
}
