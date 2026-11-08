package game

import (
	"context"
	"errors"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/slogutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redis/go-redis/v9"

	"hexchess-svc/itest"
	"hexchess-svc/utils/testutil"

	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cmpOptsChessState = cmpopts.IgnoreFields(ChessState{}, "StartTime")

func setupChessRepoTest(t slogutil.TestLogger) (*ChessRepoService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewChessRepoService(infra.Redis)

	return service, infra
}

func TestEchoChessState(t *testing.T) {
	service, testinfra := setupChessRepoTest(t)
	defer testinfra.Close()

	id1 := gameid.NewGameID()
	id2 := gameid.NewGameID()

	s1 := NewChessState(ChessStateArgs{ID: id1, Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
	ctx := t.Context()

	require.NoError(t, service.SetChessState(ctx, id1, s1))

	outState1, err := service.GetChessState(ctx, id1)
	require.NoError(t, err)

	_, errBadID := service.GetChessState(ctx, id2)

	assert.Equal(t, ErrNoChessState, errBadID)
	assert.NotNil(t, outState1)
	testutil.Equal(t, s1, outState1, cmpOptsChessState)
}

func TestUpdateChessState(t *testing.T) {
	service, testinfra := setupChessRepoTest(t)
	defer testinfra.Close()

	testID := gameid.NewGameID()
	arbitraryKey := uuid.New().String()

	inState := NewChessState(ChessStateArgs{ID: testID, Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
	ctx := t.Context()

	require.NoError(t, service.SetChessState(ctx, testID, inState))

	update := func(state *ChessState) error {
		state.EndState = Aborted // arbitrary state update
		return nil
	}
	commit := func(pipe redis.Pipeliner, state *ChessState) error {
		return pipe.Set(ctx, arbitraryKey, "test", 0).Err()
	}
	outState, err := service.UpdateChessStateTxn(ctx, testID, update, commit)
	require.NoError(t, err)

	wantState := inState.DeepCopy()
	wantState.EndState = Aborted

	testutil.Equal(t, &wantState, outState, cmpOptsChessState)
	assertRedisChess(t, testinfra.Redis, &wantState, cmpOptsChessState)

	arbitraryVal, err := testinfra.Redis.PrimaryClient.Get(ctx, arbitraryKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "test", arbitraryVal)
}

func TestUpdateChessState_Errors(t *testing.T) {
	service, testinfra := setupChessRepoTest(t)
	defer testinfra.Close()

	testID := gameid.NewGameID()

	inState := NewChessState(ChessStateArgs{ID: testID, Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.White})
	require.NoError(t, service.SetChessState(context.Background(), testID, inState))
	ctx := t.Context()

	t.Run("failing with unknown gameID", func(t *testing.T) {
		_, err := service.UpdateChessStateTxn(ctx, gameid.NewGameID(), func(state *ChessState) error { return nil }, nil)

		assert.Equal(t, ErrNoChessState, err)
	})

	t.Run("failing in error closure", func(t *testing.T) {
		mockedErr := errors.New("failed in update closure")

		_, err := service.UpdateChessStateTxn(ctx, testID, func(state *ChessState) error {
			return mockedErr
		}, nil)

		assert.Equal(t, mockedErr, err)
	})

	t.Run("failing with interrupted update", func(t *testing.T) {
		_, err := service.UpdateChessStateTxn(ctx, testID, func(state *ChessState) error {
			require.NoError(t, service.SetChessState(ctx, testID, inState)) // the state value we set is arbitrary
			return nil
		}, nil)

		assert.Equal(t, ErrMaxChessStateRetries, err)
	})
}
