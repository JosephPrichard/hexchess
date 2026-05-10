package svc

import (
	"context"
	"errors"
	"hexchess-svc/model"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"hexchess-svc/hexchess"
	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoChessState(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()

	s1 := MakeChessState(StateSetup{ID: id1, Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	require.NoError(t, services.SetChessState(ctx, id1, s1))

	outState1, err := services.GetChessState(ctx, id1)
	require.NoError(t, err)

	_, errBadID := services.GetChessState(ctx, id2)

	assert.Equal(t, ErrNoChessState, errBadID)
	assert.NotNil(t, outState1)
	testutil.Equal(t, s1, outState1, ChessMetaCmpOpt)
}

func TestUpdateChessState(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	testID := "testing-id1-" + uuid.NewString()
	arbitraryKey := uuid.NewString()

	inState := MakeChessState(StateSetup{ID: testID, Mode: model.ModeCorrespondence1, FirstColor: model.Random})

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	require.NoError(t, services.SetChessState(ctx, testID, inState))

	update := func(state *ChessState) error {
		state.EndState = Aborted // arbitrary state update
		return nil
	}
	commit := func(pipe redis.Pipeliner, state *ChessState) error {
		return pipe.Set(ctx, arbitraryKey, "test", 0).Err()
	}
	outState, err := services.updateChessStateTxn(ctx, testID, update, commit)
	require.NoError(t, err)

	wantState := inState.DeepCopy()
	wantState.EndState = Aborted

	testutil.Equal(t, &wantState, outState, ChessMetaCmpOpt)
	AssertRedisChessState(t, services, &wantState, ChessMetaCmpOpt)

	arbitraryVal, err := services.redis.Cache.Get(ctx, arbitraryKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "test", arbitraryVal)
}

func TestUpdateChessState_Errors(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	testID := "testing-id1-" + uuid.NewString()

	inState := MakeChessState(StateSetup{ID: testID, Mode: model.ModeCorrespondence1, FirstColor: model.White})
	require.NoError(t, services.SetChessState(context.Background(), testID, inState))

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	t.Run("failing with unknown game existingID", func(t *testing.T) {
		_, err := services.updateChessStateTxn(ctx, uuid.NewString(), func(state *ChessState) error { return nil }, nil)

		assert.Equal(t, ErrNoChessState, err)
	})

	t.Run("failing in error closure", func(t *testing.T) {
		mockedErr := errors.New("failed in update closure")

		_, err := services.updateChessStateTxn(ctx, testID, func(state *ChessState) error {
			return mockedErr
		}, nil)

		assert.Equal(t, mockedErr, err)
	})

	t.Run("failing with interrupted update", func(t *testing.T) {
		_, err := services.updateChessStateTxn(ctx, testID, func(state *ChessState) error {
			require.NoError(t, services.SetChessState(ctx, testID, inState)) // the state value we set is arbitrary
			return nil
		}, nil)

		assert.Equal(t, ErrMaxChessStateRetries, err)
	})
}

func TestGetChessMetas(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	id1 := "testing-id1-" + uuid.NewString()
	id2 := "testing-id2-" + uuid.NewString()
	id3 := "testing-id3-" + uuid.NewString()

	s1 := MakeChessState(StateSetup{
		ID:         id1,
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
	})
	s2 := MakeChessState(StateSetup{ID: id2, Mode: model.ModeCorrespondence1, FirstColor: model.Random, Black: model.PlayerState{ID: 1, Present: true}})
	s3 := MakeChessState(StateSetup{ID: id3, Mode: model.ModeCorrespondence1, FirstColor: model.Random, Black: model.PlayerState{ID: 1, Present: true}})

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	now := time.Now()

	// these times must be after now.Add(-GameExpireFinished)
	require.NoError(t, services.setChessStateAt(ctx, id1, s1, now.Add(-100*time.Second)))
	require.NoError(t, services.setChessStateAt(ctx, id2, s2, now.Add(-50*time.Second)))
	require.NoError(t, services.setChessStateAt(ctx, id3, s3, now.Add(-10*time.Second)))

	metaList1, err := services.GetUserChessMetas(ctx, 1)
	require.NoError(t, err)
	metaList2, err := services.GetUserChessMetas(ctx, 2)
	require.NoError(t, err)
	metaList3, err := services.GetUserChessMetas(ctx, 3)
	require.NoError(t, err)
	metaList4, err := services.GetUserChessMetasPaged(ctx, 1, 1, 2)
	require.NoError(t, err)
	metaList5, err := services.GetUserChessMetasPaged(ctx, 1, 2, 2)
	require.NoError(t, err)

	m1 := ChessMeta{
		ID:          id1,
		WhitePlayer: model.PlayerState{ID: 1, Present: true},
		BlackPlayer: model.PlayerState{ID: 2, Present: true},
		FirstColor:  model.Random, Mode: model.ModeCorrespondence1,
	}
	m2 := ChessMeta{ID: id2, BlackPlayer: model.PlayerState{ID: 1, Present: true}, FirstColor: model.Random, Mode: model.ModeCorrespondence1}
	m3 := ChessMeta{ID: id3, BlackPlayer: model.PlayerState{ID: 1, Present: true}, FirstColor: model.Random, Mode: model.ModeCorrespondence1}

	assert.Equal(t, []ChessMeta{m3, m2, m1}, metaList1)
	assert.Equal(t, []ChessMeta{m1}, metaList2)
	assert.Empty(t, metaList3)
	assert.Equal(t, []ChessMeta{m3, m2}, metaList4)
	assert.Equal(t, []ChessMeta{m1}, metaList5)
}

func TestUndo(t *testing.T) {
	t.Parallel()

	t.Run("no moves to undo", func(t *testing.T) {
		t.Parallel()

		s := MakeChessState(StateSetup{
			ID:           "test",
			Game:         ptr(hexchess.MakeStartGame()),
			InitialBoard: ptr(hexchess.InitialBoard()),
		})

		err := s.Undo()

		assert.Equal(t, ErrNoMoveUndo, err)
	})

	t.Run("successfully undoing game with one move", func(t *testing.T) {
		t.Parallel()

		game := hexchess.MakeStartGame()
		game.Moves = append(game.Moves, game.MakeMove(hexchess.Move{From: hexchess.HexStr("b1"), To: hexchess.HexStr("b2")}))

		s := MakeChessState(StateSetup{
			ID:           "test",
			Game:         ptr(game),
			InitialBoard: ptr(hexchess.InitialBoard()),
		})

		err := s.Undo()

		require.NoError(t, err)
		assert.Len(t, s.Game.Moves, 0)
	})
}
