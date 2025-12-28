package web

import (
	"context"
	"encoding/json"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/ptr"
	"hexchess-svc/services"
	"strings"
	"testing"
)

var TestSessionID1 = "test-session-id-1"
var TestSessionID2 = "test-session-id-2"
var TestGameID1 = "game1"

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json.Marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}

var TestStates = []svc.ChessState{
	svc.MakeState(svc.StateSetup{
		ID:         TestGameID1,
		Mode:       svc.ModeCorrespondence1,
		FirstColor: svc.Random,
		Black:      ptr.New(svc.MakePlayer(2, "user2", "us")),
	}),
	svc.MakeState(svc.StateSetup{ID: "game2", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
	svc.MakeState(svc.StateSetup{ID: "game3", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
}

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

	for _, state := range TestStates {
		if err := svc.SetChessState(ctx, rdb, state.ID, &state); err != nil {
			t.Fatalf("create test states: %v", err)
		}
	}
}
