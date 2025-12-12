package web

import (
	"context"
	"hexchess-svc/dpl"
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"testing"
)

func BeforeDbTxnTests(t infra.TestLogger) (infra.Databases, func()) {
	return infra.BeforeDbTests(t, true, dpl.InsertTestData)
}

func BeforeDbTests(t infra.TestLogger) (infra.Databases, func()) {
	return infra.BeforeDbTests(t, false, dpl.InsertTestData)
}

var TestSessionID1 = "test-session-id-1"
var TestSessionID2 = "test-session-id-2"

func createTestSessions(t *testing.T, rdb *infra.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-session-1")
	if err := dpl.SetSession(ctx, rdb, TestSessionID1, dpl.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
	if err := dpl.SetSession(ctx, rdb, TestSessionID2, dpl.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
}

func createTestChessStates(t *testing.T, rdb *infra.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-password")

	state1 := dpl.MakeState("game1", dpl.TcRealTime, dpl.CsRandom, nil)
	state1.WhitePlayer = &dpl.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}

	for _, state := range []dpl.ChessState{
		state1,
		dpl.MakeState("game2", dpl.TcRealTime, dpl.CsRandom, nil),
		dpl.MakeState("game3", dpl.TcRealTime, dpl.CsRandom, nil),
	} {
		if _, err := dpl.SetChessState(ctx, rdb, state.ID, state); err != nil {
			t.Fatalf("create test states: %v", err)
		}
	}
}
