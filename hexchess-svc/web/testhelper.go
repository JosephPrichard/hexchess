package web

import (
	"context"
	"encoding/json"
	"hexchess-svc/domain"
	"strings"
	"testing"

	"hexchess-svc/service"
	"hexchess-svc/util/logutil"
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

var TestStates = []*svc.ChessState{
	svc.MakeChessState(svc.StateSetup{
		ID:         TestGameID1,
		Mode:       domain.ModeCorrespondence1,
		FirstColor: domain.Random,
		Black:      domain.MakePlayer(2, "user2", "us"),
		UndoState:  svc.UndoState{UndoID: 2},
	}),
	svc.MakeChessState(svc.StateSetup{ID: "game2", Mode: domain.ModeCorrespondence1, FirstColor: domain.Random}),
	svc.MakeChessState(svc.StateSetup{ID: "game3", Mode: domain.ModeCorrespondence1, FirstColor: domain.Random}),
}

func createTestSessions(t *testing.T, services *svc.HexchessServices) {
	t.Helper()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-testing-session-1")
	if err := services.SetSessions(ctx,
		svc.SessionInst{SessionID: TestSessionID1, Player: domain.MakePlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		svc.SessionInst{SessionID: TestSessionID2, Player: domain.MakePlayer(2, "user2", "us"), Expiry: SessionMaxAge},
	); err != nil {
		t.Fatalf("create testing session: %v", err)
	}
}

func createTestChessStates(t *testing.T, services *svc.HexchessServices) {
	t.Helper()
	ctx := context.WithValue(context.Background(), logutil.Trace, "testing-update-password")
	for _, state := range TestStates {
		if err := services.SetChessState(ctx, state.ID, state); err != nil {
			t.Fatalf("create testing ss: %v", err)
		}
	}
}
