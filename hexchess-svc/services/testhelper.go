package svc

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"hexchess-svc/db"
	"hexchess-svc/out"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"testing"
)

func BeforeStateTest(t logutil.TestLogger, useTx bool) (State, func()) {
	pdb, pdbCloser := db.BeforePostgresTest(t, useTx)
	rdb := db.BeforeRedisTest(t)
	state := State{
		Postgres:      pdb,
		Redis:         rdb,
		EntropySource: &out.NDEntropySource{},
	}
	return state, func() { pdbCloser(); rdb.Close() }
}

func AssertRedisChess(t *testing.T, s State, wantState ChessState, options ...cmp.Option) {
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
