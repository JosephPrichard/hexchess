package network

import (
	"encoding/json"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"hexchess-svc/model"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/async"
)

type serviceMocks struct {
	Entropy    entropy.Generator
	Remote     cloud.SDKs
	Dispatcher async.Dispatcher
}

func setupMuxTest(t alog.TestLogger, mocks *serviceMocks) (http.Handler, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}

	infra := itest.SetupIntegrationTest(t)

	broadcaster := pubsub.NewSyncBroadcaster(infra.Redis)

	h := NewServeMux(ServeMuxSetup{
		Database:    infra.Database,
		Redis:       infra.Redis,
		AWS:         infra.AWS,
		SDKs:        mocks.Remote,
		Entropy:     mocks.Entropy,
		Dispatcher:  mocks.Dispatcher,
		Broadcaster: broadcaster,
	})

	return h, infra
}

type websocketTestContext struct {
	testinfra    itest.TestInfra
	broadcasters *pubsub.LocalBroadcasters
	testServer   *httptest.Server
}

func setupWebsocketTest(t *testing.T) websocketTestContext {
	testinfra := itest.SetupIntegrationTest(t)

	localBroadcasters := pubsub.NewLocalBroadcasters()
	<-localBroadcasters.ListenGameMessages(testinfra.Redis)

	createTestSessions(t, testinfra.Redis)
	createTestChessStates(t, testinfra.Redis)

	testServer := httptest.NewServer(NewServeMux(ServeMuxSetup{
		Database:     testinfra.Database,
		Redis:        testinfra.Redis,
		Broadcasters: localBroadcasters,
		Broadcaster:  pubsub.NewSyncBroadcaster(testinfra.Redis),
		Dispatcher:   async.SyncDispatcher{},
	}))
	return websocketTestContext{testinfra: testinfra, broadcasters: localBroadcasters, testServer: testServer}
}

func (s *websocketTestContext) getWsURL() string {
	return strings.Replace(s.testServer.URL, "http", "ws", 1)
}

func (s *websocketTestContext) Shutdown() {
	s.testinfra.Close()
	s.testServer.Close()
}

type sseTestContext struct {
	testinfra         itest.TestInfra
	localBroadcasters *pubsub.LocalBroadcasters
	broadcaster       pubsub.Broadcaster
	testServer        *httptest.Server
}

func (s *sseTestContext) Shutdown() {
	s.testinfra.Close()
	s.testServer.Close()
}

func setupSSETest(t *testing.T) sseTestContext {
	testinfra := itest.SetupIntegrationTest(t)

	createTestSessions(t, testinfra.Redis)

	localBroadcasters := pubsub.NewLocalBroadcasters()

	<-localBroadcasters.ListenCountEvents(testinfra.Redis)
	<-localBroadcasters.ListenUsersMessages(testinfra.Redis)
	<-localBroadcasters.ListenTournamentMessages(testinfra.Redis)

	testServer := httptest.NewServer(NewServeMux(ServeMuxSetup{
		Database:     testinfra.Database,
		Redis:        testinfra.Redis,
		Entropy:      &entropy.StableSource{},
		Dispatcher:   async.SyncDispatcher{},
		Broadcaster:  pubsub.NewSyncBroadcaster(testinfra.Redis),
		Broadcasters: localBroadcasters,
	}))
	return sseTestContext{
		testinfra:         testinfra,
		broadcaster:       pubsub.NewSyncBroadcaster(testinfra.Redis),
		localBroadcasters: localBroadcasters,
		testServer:        testServer,
	}
}

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = model.NewGameID()

func createTestSessions(t *testing.T, redis cache.Redis) {
	t.Helper()
	ctx := t.Context()

	for _, session := range []struct {
		ID     string
		Player model.PlayerState
		Expiry time.Duration
	}{
		{ID: TestSessionID1, Player: model.NewPlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		{ID: TestSessionID2, Player: model.NewPlayer(2, "user2", "us"), Expiry: SessionMaxAge},
	} {
		data, err := model.MarshalPlayer(session.Player)
		if err != nil {
			t.Fatalf("marshal session: %v", err)
		}
		sessionKey := fmt.Sprintf("session:{%s}", session.ID)
		if err := redis.PrimaryClient.SetEx(ctx, sessionKey, data, session.Expiry).Err(); err != nil {
			t.Fatalf("set session: %v", err)
		}
	}
}

func createTestChessStates(t *testing.T, redis cache.Redis) {
	t.Helper()

	var testStates = []*model.ChessState{
		model.NewChessState(model.StateSetup{
			ID:         TestGameID1,
			Mode:       model.ModeCorrespondence1,
			FirstColor: model.Random,
			Black:      model.NewPlayer(2, "user2", "us"),
			UndoState:  model.UndoState{UndoID: 2},
		}),
		model.NewChessState(model.StateSetup{ID: "game2", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
		model.NewChessState(model.StateSetup{ID: "game3", Mode: model.ModeCorrespondence1, FirstColor: model.Random}),
	}
	ctx := t.Context()
	for _, state := range testStates {
		gameKey := fmt.Sprintf("game:%s{%c}", state.ID, state.ID.Partition())

		bytes, err := model.MarshalChessState(state)
		if err != nil {
			t.Fatalf("marshal chess state: %v", err)
		}
		if err := redis.PrimaryClient.Set(ctx, gameKey, bytes, 0).Err(); err != nil {
			t.Fatalf("set chess state: %v", err)
		}
	}
}

type updtLbChangeSet struct {
	Mode    model.GameMode
	ID      int64
	EloDiff float64
}

func createLeaderboard(t *testing.T, rdb cache.Redis, changes ...updtLbChangeSet) {
	ctx := t.Context()
	pipe := rdb.PrimaryClient.Pipeline()
	for _, change := range changes {
		modeLbZSet := fmt.Sprintf("%s/mode:{%s}", cache.Constants.LeaderboardZSet, change.Mode.String())
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("failed to create leaderboard: %v", err)
	}
}

func asJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}
