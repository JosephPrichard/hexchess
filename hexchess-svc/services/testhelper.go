package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"testing"
)

func AssertRedisChess(t *testing.T, rdb *db.Redis, wantState ChessState) {
	ctx := context.WithValue(t.Context(), logutil.Trace, "assert-chess-states")
	actualState, err := GetChessState(ctx, rdb, wantState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	assertutil.AssertEqualIgnoring(t, wantState, *actualState, ChessMetaCmpOpts)
}

func AssertChessState(t *testing.T, wantState ChessState, actualState *ChessState) {
	if actualState == nil {
		t.Fatalf("chess state is nil")
	}
	assertutil.AssertEqualIgnoring(t, wantState, *actualState, ChessMetaCmpOpts)
}
