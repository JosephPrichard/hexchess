package web

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/svc"
	"hexchess-svc/util"
	"testing"
)

var TestSessionID1 = "test-session-id-1"
var TestSessionID2 = "test-session-id-2"

func createTestSessions(t *testing.T, rdb *db.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-session-1")
	if err := svc.SetSession(ctx, rdb, TestSessionID1, svc.PlayerState{ID: 1, Name: "user1", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
	if err := svc.SetSession(ctx, rdb, TestSessionID2, svc.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}, SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
}

func createTestChessStates(t *testing.T, rdb *db.Redis) {
	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-password")

	state1 := svc.MakeState(svc.StateSetup{ID: "game1", TimeControl: svc.TcRealTime, FirstColor: svc.ColorRandom})
	state1.WhitePlayer = &svc.PlayerState{ID: 2, Name: "user2", Country: "us", Elo: 1000}

	for _, state := range []svc.ChessState{
		state1,
		svc.MakeState(svc.StateSetup{ID: "game2", TimeControl: svc.TcRealTime, FirstColor: svc.ColorRandom}),
		svc.MakeState(svc.StateSetup{ID: "game3", TimeControl: svc.TcRealTime, FirstColor: svc.ColorRandom}),
	} {
		if _, err := svc.SetChessState(ctx, rdb, state.ID, state); err != nil {
			t.Fatalf("create test states: %v", err)
		}
	}
}
