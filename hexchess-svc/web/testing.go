package web

import (
	"context"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"testing"
)

func createTestSessions(t *testing.T, rdb data.Rdb) string {
	sessionID := "test-session-id"
	ctx := context.WithValue(context.Background(), lib.TraceKey, "create-test-sessions")
	if err := data.SetSession(ctx, rdb, sessionID, data.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, MaxAgeCookie); err != nil {
		t.Fatalf("failed to create test sessions: %v", err)
	}
	return sessionID
}

func createTestChessStates(t *testing.T, rdb data.Rdb) {
	ctx := context.WithValue(context.Background(), lib.TraceKey, "testing-update-password")
	for _, state := range []data.ChessState{
		data.MakeStateWithPlayers("game1", data.RealTime, nil, &data.PlayerState{ID: 1, Name: "username", Country: "us", Elo: 1000}),
		data.MakeState("game2", data.RealTime),
		data.MakeState("game3", data.RealTime),
	} {
		if _, err := data.SetChessState(ctx, rdb, state.ID, state); err != nil {
			t.Fatalf("failed to create test states: %v", err)
		}
	}
}
