package svc

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"

	"slices"
	"testing"
)

func SetupStateTest(t logutil.TestLogger, flags ...itest.TestFlag) State {
	var state State
	var eg errgroup.Group

	if slices.Contains(flags, itest.WithPostgres) {
		eg.Go(func() (err error) {
			useTxn := slices.Contains(flags, itest.UseTxn)
			state.Postgres, err = itest.SetupPostgresTest(t, useTxn)
			return
		})
	}
	if slices.Contains(flags, itest.WithRedis) {
		eg.Go(func() (err error) {
			state.Redis, err = itest.SetupRedisTest(t)
			return
		})
	}
	if slices.Contains(flags, itest.WithAws) {
		eg.Go(func() (err error) {
			state.Aws, err = itest.SetupAwsTest(t)
			return
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to setup test state: %v", err)
	}

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
