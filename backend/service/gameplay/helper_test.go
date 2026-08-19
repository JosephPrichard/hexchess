package gameplay

import (
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

var cmpOptsChessState = cmpopts.IgnoreFields(model.ChessState{}, "StartTime")

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *model.ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := t.Context()

	if wantState == nil {
		return
	}
	actualState, err := gamestate.NewChessRepoService(redis).GetChessState(ctx, wantState.ID)
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
		require.NoError(t, gamestate.NewChessRepoService(redis).SetChessState(t.Context(), g.ID, g))
	}
}
