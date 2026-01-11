package web

import (
	"context"
	"encoding/json"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/ptr"
	"hexchess-svc/services"
	"strings"
	"testing"
)

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = "game1"

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}

var TestStates = []svc.ChessState{
	svc.MakeChess(svc.StateSetup{
		ID:         TestGameID1,
		Mode:       svc.ModeCorrespondence1,
		FirstColor: svc.Random,
		Black:      ptr.New(svc.MakePlayer(2, "user2", "us")),
	}),
	svc.MakeChess(svc.StateSetup{ID: "game2", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
	svc.MakeChess(svc.StateSetup{ID: "game3", Mode: svc.ModeCorrespondence1, FirstColor: svc.Random}),
}

func createTestSessions(t *testing.T, s svc.State) {
	t.Helper()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-testing-session-1")
	if err := s.SetSessions(ctx,
		svc.SessInst{SessionID: TestSessionID1, Player: svc.MakePlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		svc.SessInst{SessionID: TestSessionID2, Player: svc.MakePlayer(2, "user2", "us"), Expiry: SessionMaxAge},
	); err != nil {
		t.Fatalf("create testing session: %v", err)
	}
}

func createTestChessStates(t *testing.T, s svc.State) {
	t.Helper()
	ctx := context.WithValue(context.Background(), logutil.Trace, "testing-update-password")
	for _, state := range TestStates {
		if err := s.SetChessState(ctx, state.ID, &state); err != nil {
			t.Fatalf("create testing states: %v", err)
		}
	}
}
