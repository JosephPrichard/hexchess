package web

import (
	"context"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"testing"
)

var TestSessionID1 = "test-session-id-1"
var TestSessionID2 = "test-session-id-2"

func createTestSessions(t *testing.T, rdb data.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-session-1")
	if err := data.SetSession(ctx, rdb, TestSessionID1, data.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("failed to create test sessions: %v", err)
	}
	if err := data.SetSession(ctx, rdb, TestSessionID2, data.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("failed to create test sessions: %v", err)
	}
}

func createTestChessStates(t *testing.T, rdb data.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-password")
	for _, state := range []data.ChessState{
		data.MakeStateWithPlayers("game1", data.RealTime, &data.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}, nil),
		data.MakeState("game2", data.RealTime),
		data.MakeState("game3", data.RealTime),
	} {
		if _, err := data.SetChessState(ctx, rdb, state.ID, state); err != nil {
			t.Fatalf("failed to create test states: %v", err)
		}
	}
}
