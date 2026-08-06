package gamestate

import (
	"context"
	"errors"
	"hexchess-svc/model"
	"hexchess-svc/utils/logutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"

	"hexchess-svc/itest"
	"hexchess-svc/utils/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRepoTest(t logutil.TestLogger, flags ...itest.TestFlag) (*ChessRepoService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewChessRepoService(infra.Redis)

	return services, infra
}

func assertRedisChess(t *testing.T, services *ChessRepoService, wantState *model.ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := t.Context()

	if wantState == nil {
		return
	}
	actualState, err := services.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("failed to retrieve in redis chess state assert: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func TestEchoChessState(t *testing.T) {
	t.Parallel()

	services, testinfra := setupRepoTest(t, itest.Redis)
	defer testinfra.Close()

	id1 := model.NewGameID()
	id2 := model.NewGameID()

	s1 := model.NewChessState(model.StateSetup{ID: id1, Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	ctx := t.Context()

	require.NoError(t, services.SetChessState(ctx, id1, s1))

	outState1, err := services.GetChessState(ctx, id1)
	require.NoError(t, err)

	_, errBadID := services.GetChessState(ctx, id2)

	assert.Equal(t, ErrNoChessState, errBadID)
	assert.NotNil(t, outState1)
	testutil.Equal(t, s1, outState1)
}

func TestUpdateChessState(t *testing.T) {
	t.Parallel()

	services, testinfra := setupRepoTest(t, itest.Redis)
	defer testinfra.Close()

	testID := model.NewGameID()
	arbitraryKey := uuid.NewString()

	inState := model.NewChessState(model.StateSetup{ID: testID, Mode: model.ModeCorrespondence1, FirstColor: model.Random})

	ctx := t.Context()

	require.NoError(t, services.SetChessState(ctx, testID, inState))

	update := func(state *model.ChessState) error {
		state.EndState = model.Aborted // arbitrary state update
		return nil
	}
	commit := func(pipe redis.Pipeliner, state *model.ChessState) error {
		return pipe.Set(ctx, arbitraryKey, "test", 0).Err()
	}
	outState, err := services.UpdateChessStateTxn(ctx, testID, update, commit)
	require.NoError(t, err)

	wantState := inState.DeepCopy()
	wantState.EndState = model.Aborted

	testutil.Equal(t, &wantState, outState)
	assertRedisChess(t, services, &wantState)

	arbitraryVal, err := testinfra.Redis.PrimaryClient.Get(ctx, arbitraryKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "test", arbitraryVal)
}

func TestUpdateChessState_Errors(t *testing.T) {
	t.Parallel()

	services, testinfra := setupRepoTest(t, itest.Redis)
	defer testinfra.Close()

	testID := model.NewGameID()

	inState := model.NewChessState(model.StateSetup{ID: testID, Mode: model.ModeCorrespondence1, FirstColor: model.White})
	require.NoError(t, services.SetChessState(context.Background(), testID, inState))

	ctx := t.Context()

	t.Run("failing with unknown gameID", func(t *testing.T) {
		_, err := services.UpdateChessStateTxn(ctx, model.NewGameID(), func(state *model.ChessState) error { return nil }, nil)

		assert.Equal(t, ErrNoChessState, err)
	})

	t.Run("failing in error closure", func(t *testing.T) {
		mockedErr := errors.New("failed in update closure")

		_, err := services.UpdateChessStateTxn(ctx, testID, func(state *model.ChessState) error {
			return mockedErr
		}, nil)

		assert.Equal(t, mockedErr, err)
	})

	t.Run("failing with interrupted update", func(t *testing.T) {
		_, err := services.UpdateChessStateTxn(ctx, testID, func(state *model.ChessState) error {
			require.NoError(t, services.SetChessState(ctx, testID, inState)) // the state value we set is arbitrary
			return nil
		}, nil)

		assert.Equal(t, ErrMaxChessStateRetries, err)
	})
}
