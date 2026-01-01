package svc

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"testing"
)

func AssertRedisChess(t *testing.T, s State, wantState ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := context.WithValue(t.Context(), logutil.Trace, "assert-chess-states")
	actualState, err := s.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	assertutil.AssertEqualIgnoring(t, wantState, *actualState, options...)
}

func AssertChessState(t *testing.T, wantState ChessState, actualState *ChessState, options ...cmp.Option) {
	t.Helper()
	if actualState == nil {
		t.Fatalf("chess state is nil")
	}
	assertutil.AssertEqualIgnoring(t, wantState, *actualState, options...)
}
