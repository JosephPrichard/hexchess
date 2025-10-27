package web

import (
	"context"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"testing"
)

var sessionID1 = "test-session-id-1"
var sessionID2 = "test-session-id-2"

func createTestSessions(t *testing.T, rdb data.Redis) {
	ctx := context.WithValue(context.Background(), lib.TK, "create-test-session-1")
	if err := data.SetSession(ctx, rdb, sessionID1, data.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("failed to create test sessions: %v", err)
	}
	if err := data.SetSession(ctx, rdb, sessionID2, data.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("failed to create test sessions: %v", err)
	}
}

func createTestChessStates(t *testing.T, rdb data.Redis) {
	ctx := context.WithValue(context.Background(), lib.TK, "testing-update-password")
	for _, state := range []data.ChessState{
		data.MakeStateWithPlayers("game1", data.RealTime,
			&data.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000},
			nil),
		data.MakeState("game2", data.RealTime),
		data.MakeState("game3", data.RealTime),
	} {
		if _, err := data.SetChessState(ctx, rdb, state.ID, state); err != nil {
			t.Fatalf("failed to create test states: %v", err)
		}
	}
}
