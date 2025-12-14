package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEchoChessState(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()

	state1 := MakeState(StateSetup{ID: id1, TimeControl: TcRealTime, FirstColor: ColorRandom})
	ctx := context.WithValue(context.Background(), util.Trace, "testing-set-then-get")

	// when
	_, err := SetChessState(ctx, rdb, id1, state1)
	assert.NoError(t, err)

	outState1, err := GetChessState(ctx, rdb, id1)
	assert.NoError(t, err)

	_, errBadID := GetChessState(ctx, rdb, id2)

	// then
	assert.Equal(t, ErrNoChessState, errBadID)
	util.AssertEqualIgnoring(t, state1, outState1, ChessMetaCmpOpts)
}

func TestGetChessMetas(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	state1 := MakeState(StateSetup{ID: id1, TimeControl: TcRealTime, FirstColor: ColorRandom, White: &PlayerState{ID: 1}, Black: &PlayerState{ID: 2}})
	state2 := MakeState(StateSetup{ID: id2, TimeControl: TcRealTime, FirstColor: ColorRandom, Black: &PlayerState{ID: 1}})
	state3 := MakeState(StateSetup{ID: id3, TimeControl: TcRealTime, FirstColor: ColorRandom, Black: &PlayerState{ID: 1}})

	ctx := context.WithValue(context.Background(), util.Trace, "testing-get-metas")
	now := time.Now()

	// when
	// these times must be after now.Add(-GameExpireFinished)
	_, err := SetChessStateAt(ctx, rdb, id1, state1, now.Add(-100*time.Second))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id2, state2, now.Add(-50*time.Second))
	assert.NoError(t, err)
	_, err = SetChessStateAt(ctx, rdb, id3, state3, now.Add(-10*time.Second))
	assert.NoError(t, err)

	metaList1, err := GetUserChessMetas(ctx, rdb, 1)
	assert.NoError(t, err)
	metaList2, err := GetUserChessMetas(ctx, rdb, 2)
	assert.NoError(t, err)
	metaList3, err := GetUserChessMetas(ctx, rdb, 3)
	assert.NoError(t, err)
	metaList4, err := GetUserChessMetasPaged(ctx, rdb, 1, 1, 2)
	assert.NoError(t, err)
	metaList5, err := GetUserChessMetasPaged(ctx, rdb, 1, 2, 2)
	assert.NoError(t, err)

	// then
	m1 := ChessMeta{ID: id1, WhitePlayer: &PlayerState{ID: 1}, BlackPlayer: &PlayerState{ID: 2}, FirstColor: ColorRandom, TimeControl: TcRealTime}
	m2 := ChessMeta{ID: id2, BlackPlayer: &PlayerState{ID: 1}, FirstColor: ColorRandom, TimeControl: TcRealTime}
	m3 := ChessMeta{ID: id3, BlackPlayer: &PlayerState{ID: 1}, FirstColor: ColorRandom, TimeControl: TcRealTime}

	assert.Equal(t, []ChessMeta{m3, m2, m1}, metaList1)
	assert.Equal(t, []ChessMeta{m1}, metaList2)
	assert.Empty(t, metaList3)
	assert.Equal(t, []ChessMeta{m3, m2}, metaList4)
	assert.Equal(t, []ChessMeta{m1}, metaList5)
}

func TestExpireChessStates(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()

	state1 := MakeState(StateSetup{ID: id1, TimeControl: TcRealTime, FirstColor: ColorRandom})

	state1.WhitePlayer = &PlayerState{ID: 1}
	state1.BlackPlayer = &PlayerState{ID: 2}

	ctx := context.WithValue(context.Background(), util.Trace, "testing-expire")
	now := time.Now()

	// when
	// these times must be before now.Add(-GameExpireFinished)
	_, err := SetChessStateAt(ctx, rdb, id1, state1, now.Add(2*-GameExpireFinished))
	assert.NoError(t, err)

	err = ExpireChessStates(ctx, rdb, rdb.GamesZSet)
	assert.NoError(t, err)

	_, err = GetChessState(ctx, rdb, id1)

	// then
	assert.Equal(t, ErrNoChessState, err)
}
