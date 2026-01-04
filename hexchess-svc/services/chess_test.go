package svc

import (
	"context"
	"github.com/stretchr/testify/require"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/ptr"
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

	state1 := MakeState(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random})
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb}

	// when
	require.NoError(t, s.SetChessState(ctx, id1, &state1))

	outState1, err := s.GetChessState(ctx, id1)
	require.NoError(t, err)

	_, errBadID := s.GetChessState(ctx, id2)

	// then
	assert.Equal(t, ErrNoChessState, errBadID)
	assertutil.Equal(t, state1, *outState1, ChessMetaCmpOpt)
}

func TestGetChessMetas(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	state1 := MakeState(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random, White: ptr.New(MakeIDPlayer(1)), Black: ptr.New(MakeIDPlayer(2))})
	state2 := MakeState(StateSetup{ID: id2, Mode: ModeCorrespondence1, FirstColor: Random, Black: ptr.New(MakeIDPlayer(1))})
	state3 := MakeState(StateSetup{ID: id3, Mode: ModeCorrespondence1, FirstColor: Random, Black: ptr.New(MakeIDPlayer(1))})

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb}
	now := time.Now()

	// when
	// these times must be after now.Add(-GameExpireFinished)
	require.NoError(t, s.SetChessStateAt(ctx, id1, &state1, now.Add(-100*time.Second)))
	require.NoError(t, s.SetChessStateAt(ctx, id2, &state2, now.Add(-50*time.Second)))
	require.NoError(t, s.SetChessStateAt(ctx, id3, &state3, now.Add(-10*time.Second)))

	metaList1, err := s.GetUserChessMetas(ctx, 1)
	require.NoError(t, err)
	metaList2, err := s.GetUserChessMetas(ctx, 2)
	require.NoError(t, err)
	metaList3, err := s.GetUserChessMetas(ctx, 3)
	require.NoError(t, err)
	metaList4, err := s.GetUserChessMetasPaged(ctx, 1, 1, 2)
	require.NoError(t, err)
	metaList5, err := s.GetUserChessMetasPaged(ctx, 1, 2, 2)
	require.NoError(t, err)

	// then
	m1 := ChessMeta{ID: id1, WhitePlayer: MakeIDPlayer(1), BlackPlayer: MakeIDPlayer(2), FirstColor: Random, Mode: ModeCorrespondence1}
	m2 := ChessMeta{ID: id2, BlackPlayer: MakeIDPlayer(1), FirstColor: Random, Mode: ModeCorrespondence1}
	m3 := ChessMeta{ID: id3, BlackPlayer: MakeIDPlayer(1), FirstColor: Random, Mode: ModeCorrespondence1}

	assert.Equal(t, []ChessMeta{m3, m2, m1}, metaList1)
	assert.Equal(t, []ChessMeta{m1}, metaList2)
	assert.Empty(t, metaList3)
	assert.Equal(t, []ChessMeta{m3, m2}, metaList4)
	assert.Equal(t, []ChessMeta{m1}, metaList5)
}

func TestEchoStateChats(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb}

	chatsIn := []StateChat{
		{
			Player: PlayerState{ID: 1, Name: "name", Country: "us", IsGuest: false, Present: true},
			SentAt: time.Date(2022, 1, 1, 0, 1, 0, 0, time.UTC),
		},
		{SentAt: time.Date(2022, 1, 1, 0, 2, 0, 0, time.UTC)},
		{SentAt: time.Date(2022, 1, 1, 0, 3, 0, 0, time.UTC)},
	}

	// when
	for _, chat := range chatsIn {
		require.NoError(t, s.InsertStateChat(ctx, id1, chat))
	}

	chatsOut, err := s.GetStateChats(ctx, id1, 3)
	require.NoError(t, err)

	// then
	assert.Equal(t, []StateChat{chatsIn[2], chatsIn[1], chatsIn[0]}, chatsOut)
}

func TestExpireChessStates(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	id1 := "testing-id1-" + uuid.NewString()

	state1 := MakeState(StateSetup{ID: id1, Mode: ModeCorrespondence1, FirstColor: Random})

	state1.WhitePlayer = MakeIDPlayer(1)
	state1.BlackPlayer = MakeIDPlayer(2)

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb}
	now := time.Now()

	// when
	// these times must be before now.Add(-GameExpireFinished)
	require.NoError(t, s.SetChessStateAt(ctx, id1, &state1, now.Add(2*-GameExpireFinished)))
	require.NoError(t, s.InsertStateChat(ctx, id1, StateChat{}))
	require.NoError(t, s.ExpireChessStates(ctx, rdb.GamesZSet))

	_, errExpiredID := s.GetChessState(ctx, id1)
	chats, err := s.GetStateChats(ctx, id1, 1)
	require.NoError(t, err)

	// then
	assert.Equal(t, ErrNoChessState, errExpiredID)
	assert.Empty(t, chats)
}

func TestChessState_UndoMove(t *testing.T) {
	for _, test := range []struct {
		name      string
		setup     func() *ChessState
		wantErr   error
		wantMoves int
	}{
		{
			name: "no moves to undo",
			setup: func() *ChessState {
				return &ChessState{InitialBoard: chess.MakeStartBoard(), Game: chess.MakeStartGame()}
			},
			wantErr:   ErrNoMoveUndo,
			wantMoves: 0,
		},
		{
			name: "successfully undo-ing move",
			setup: func() *ChessState {
				state := &ChessState{
					InitialBoard: chess.MakeStartBoard(),
					Game:         chess.MakeStartGame(),
				}
				state.Game.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")})
				return state
			},
			wantErr:   nil,
			wantMoves: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cs := test.setup()

			err := cs.UndoMove()

			if test.wantErr != nil {
				assert.Equal(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, cs.Game.Moves, test.wantMoves)
		})
	}
}
