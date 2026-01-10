package svc

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"

	"slices"
	"sync"
	"testing"
)

func SetupStateTest(t logutil.TestLogger, flags ...itest.TestFlag) State {
	var state State
	var wg sync.WaitGroup

	if slices.Contains(flags, itest.WithPostgres) {
		wg.Add(1)
		go func() {
			useTxn := slices.Contains(flags, itest.UseTxn)
			state.Postgres = itest.SetupPostgresTest(t, useTxn)
			wg.Done()
		}()
	}
	if slices.Contains(flags, itest.WithRedis) {
		wg.Add(1)
		go func() {
			state.Redis = itest.SetupRedisTest(t)
			wg.Done()
		}()
	}
	if slices.Contains(flags, itest.WithAws) {
		wg.Add(1)
		go func() {
			state.Aws = itest.SetupAwsTest(t)
			wg.Done()
		}()
	}

	wg.Wait()

	state.EntropySource = &ext.NDEntropySource{}
	return state
}

func AssertRedisChess(t *testing.T, s *State, wantState ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	actualState, err := s.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	assertutil.Equal(t, wantState, *actualState, options...)
}

func AssertChessState(t *testing.T, wantState ChessState, actualState *ChessState, options ...cmp.Option) {
	t.Helper()
	if actualState == nil {
		t.Fatalf("chess state is nil")
	}
	assertutil.Equal(t, wantState, *actualState, options...)
}
