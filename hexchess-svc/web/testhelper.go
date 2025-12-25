package web

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"testing"
)

var TestSessionID1 = "test-session-id-1"
var TestSessionID2 = "test-session-id-2"

func createTestSessions(t *testing.T, rdb *db.Redis) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-test-session-1")
	if err := svc.SetSession(ctx, rdb, TestSessionID1, svc.MakePlayer(1, "user1", "us"), SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
	if err := svc.SetSession(ctx, rdb, TestSessionID2, svc.MakePlayer(2, "user2", "us"), SessionMaxAge); err != nil {
		t.Fatalf("create test sessions: %v", err)
	}
}

func createTestChessStates(t *testing.T, rdb *db.Redis) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "testing-update-password")

	state1 := svc.MakeState(svc.StateSetup{ID: "game1", Mode: svc.ModeCorrespondence1, FirstColor: svc.ColorRandom})
	state1.WhitePlayer = svc.MakePlayer(2, "user2", "us")

	for _, state := range []svc.ChessState{
		state1,
		svc.MakeState(svc.StateSetup{ID: "game2", Mode: svc.ModeCorrespondence1, FirstColor: svc.ColorRandom}),
		svc.MakeState(svc.StateSetup{ID: "game3", Mode: svc.ModeCorrespondence1, FirstColor: svc.ColorRandom}),
	} {
		if err := svc.SetChessState(ctx, rdb, state.ID, &state); err != nil {
			t.Fatalf("create test states: %v", err)
		}
	}
}
