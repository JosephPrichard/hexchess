package api

import (
	"context"
	"encoding/json"
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hexchess-svc/lib/logutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
)

type serviceMocks struct {
	Entropy  svc.EntropyAPI
	Remote   egress.RemoteAPIs
	S3Client egress.S3ClientAPI
}

func setupTestHandler(t logutil.TestLogger, mocks serviceMocks, flags ...itest.TestFlag) (http.Handler, *svc.HexchessServices) {
	infra := itest.SetupTestInfra(t, flags...)

	broadcaster := pubsub.MakeBroadcaster(infra.Redis)

	services := svc.MakeHexchessServices(svc.Setup{
		DB:          infra.DB,
		Redis:       infra.Redis,
		AWS:         egress.AWS{S3Client: mocks.S3Client},
		Remote:      mocks.Remote,
		Entropy:     mocks.Entropy,
		Broadcaster: broadcaster,
	})
	h := MakeServeMux(ServerSetup{Services: services, Broadcaster: broadcaster})

	return h, services
}

type websocketTestContext struct {
	services          *svc.HexchessServices
	localBroadcasters *pubsub.LocalBroadcasters
	testServer        *httptest.Server
}

func setupWebsocketTest(t *testing.T) websocketTestContext {
	testinfra := itest.SetupTestInfra(t, itest.RWPostgres, itest.Redis)
	services := svc.MakeHexchessServices(svc.Setup{
		DB:          testinfra.DB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.MakeBroadcaster(testinfra.Redis),
	})

	broadcaster := pubsub.MakeBroadcaster(testinfra.Redis)

	localBroadcasters := pubsub.MakeLocalBroadcasters()
	<-localBroadcasters.ListenGameMessages(testinfra.Redis)

	createTestSessions(t, services)
	createTestChessStates(t, services)

	testServer := httptest.NewServer(MakeServeMux(ServerSetup{Services: services, Broadcasters: localBroadcasters, Broadcaster: broadcaster}))

	return websocketTestContext{services: services, localBroadcasters: localBroadcasters, testServer: testServer}
}

func (s *websocketTestContext) getWsURL() string {
	return strings.Replace(s.testServer.URL, "http", "ws", 1)
}

func (s *websocketTestContext) Shutdown() {
	s.services.Close()
	s.localBroadcasters.Shutdown()
	s.testServer.Close()
}

type sseTestContext struct {
	services          *svc.HexchessServices
	localBroadcasters *pubsub.LocalBroadcasters
	broadcasters      pubsub.BroadcasterAPI
	testServer        *httptest.Server
}

func (s *sseTestContext) Shutdown() {
	s.services.Close()
	s.localBroadcasters.Shutdown()
	s.testServer.Close()
}

func setupSSETest(t *testing.T) sseTestContext {
	testinfra := itest.SetupTestInfra(t, itest.Redis)
	services := svc.MakeHexchessServices(svc.Setup{
		DB:          testinfra.DB,
		Redis:       testinfra.Redis,
		Entropy:     &svc.StableEntropySource{},
		Broadcaster: pubsub.MakeBroadcaster(testinfra.Redis),
	})

	broadcaster := pubsub.MakeBroadcaster(testinfra.Redis)

	createTestSessions(t, services)

	localBroadcasters := pubsub.MakeLocalBroadcasters()

	<-localBroadcasters.ListenGlobalEvents(testinfra.Redis)
	<-localBroadcasters.ListenUsersMessages(testinfra.Redis)
	<-localBroadcasters.ListenTournamentMessages(testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(ServerSetup{Services: services, Broadcaster: broadcaster, Broadcasters: localBroadcasters}))

	return sseTestContext{services: services, broadcasters: broadcaster, localBroadcasters: localBroadcasters, testServer: testServer}
}

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = "game1"

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

	var testStates = []*model.ChessState{
		model.MakeChessState(model.StateSetup{
			ID:         TestGameID1,
			Mode:       model.ModeCorrespondence1,
			FirstColor: model.Random,
			Black:      model.MakePlayer(2, "user2", "us"),
			UndoState:  model.UndoState{UndoID: 2},
		}),
		model.MakeChessState(model.StateSetup{ID: "game2", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
		model.MakeChessState(model.StateSetup{ID: "game3", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
	}

	ctx := context.WithValue(context.Background(), logutil.Trace, "testing-update-password")
	for _, state := range testStates {
		if err := services.SetChessState(ctx, state.ID, state); err != nil {
			t.Fatalf("create testing ss: %v", err)
		}
	}
}

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}
