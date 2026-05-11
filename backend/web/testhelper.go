package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"hexchess-svc/internal/logutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
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
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		Black:      model.MakePlayer(2, "user2", "us"),
		UndoState:  svc.UndoState{UndoID: 2},
	}),
	svc.MakeChessState(svc.StateSetup{ID: "game2", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
	svc.MakeChessState(svc.StateSetup{ID: "game3", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
}

func createTestSessions(t *testing.T, services *svc.HexchessServices) {
	t.Helper()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-testing-session-1")
	if err := services.SetSessions(ctx,
		svc.SessionInst{SessionID: TestSessionID1, Player: model.MakePlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		svc.SessionInst{SessionID: TestSessionID2, Player: model.MakePlayer(2, "user2", "us"), Expiry: SessionMaxAge},
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
