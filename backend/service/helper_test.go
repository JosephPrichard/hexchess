package service

import (
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *model.ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := t.Context()

	if wantState == nil {
		return
	}
	actualState, err := NewChessRepoService(redis).GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("failed to retrieve in redis chess state assert: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func mutateGame(state *model.ChessState, fn func(s *model.ChessState)) *model.ChessState {
	s := state.DeepCopy()
	fn(&s)
	return &s
}

func setChessStates(t *testing.T, redis cache.Redis, games ...*model.ChessState) {
	for _, g := range games {
		require.NoError(t, NewChessRepoService(redis).SetChessState(t.Context(), g.ID, g))
	}
}
