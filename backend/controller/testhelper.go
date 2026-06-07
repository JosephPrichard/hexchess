package controller

import (
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hexchess-svc/lib/logutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
)

type serviceMocks struct {
	Entropy  svc.EntropyAPI
	Remote   egress.RemoteAPIs
	S3Client egress.S3ClientAPI
}

func setupTestHandler(t logutil.TestLogger, mocks serviceMocks, flags ...itest.TestFlag) (http.Handler, itest.TestInfra) {
	infra := itest.SetupTestInfra(t, flags...)

	broadcaster := pubsub.MakeBroadcaster(infra.Redis)

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          infra.DB,
		Redis:       infra.Redis,
		AWS:         egress.AWS{S3Client: mocks.S3Client},
		Remote:      mocks.Remote,
		Entropy:     mocks.Entropy,
		Broadcaster: broadcaster,
	})
	h := MakeServeMux(ServerSetup{Services: services, Broadcaster: broadcaster})

	return h, infra
}

type websocketTestContext struct {
	services          *svc.HexchessServices
	localBroadcasters *pubsub.LocalBroadcasters
	testServer        *httptest.Server
}

func setupWebsocketTest(t *testing.T) websocketTestContext {
	testinfra := itest.SetupTestInfra(t, itest.RWPostgres, itest.Redis)
	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.MakeBroadcaster(testinfra.Redis),
	})
	localBroadcasters := pubsub.MakeLocalBroadcasters()
	<-localBroadcasters.ListenGameMessages(testinfra.Redis)

	createTestSessions(t, testinfra.Redis)
	createTestChessStates(t, testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(ServerSetup{
		Services:     services,
		Broadcasters: localBroadcasters,
		Broadcaster:  pubsub.MakeBroadcaster(testinfra.Redis),
	}))
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
	broadcaster       pubsub.BroadcasterAPI
	testServer        *httptest.Server
}

func (s *sseTestContext) Shutdown() {
	s.services.Close()
	s.localBroadcasters.Shutdown()
	s.testServer.Close()
}

func setupSSETest(t *testing.T) sseTestContext {
	testinfra := itest.SetupTestInfra(t, itest.ROPostgres, itest.Redis)
	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          testinfra.DB,
		Querier:     testinfra.Querier,
		Redis:       testinfra.Redis,
		Entropy:     &svc.StableEntropySource{},
		Broadcaster: pubsub.MakeBroadcaster(testinfra.Redis),
	})

	createTestSessions(t, testinfra.Redis)

	localBroadcasters := pubsub.MakeLocalBroadcasters()

	<-localBroadcasters.ListenGlobalEvents(testinfra.Redis)
	<-localBroadcasters.ListenUsersMessages(testinfra.Redis)
	<-localBroadcasters.ListenTournamentMessages(testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(ServerSetup{
		Services:     services,
		Broadcaster:  pubsub.MakeBroadcaster(testinfra.Redis),
		Broadcasters: localBroadcasters,
	}))
	return sseTestContext{
		services:          services,
		broadcaster:       pubsub.MakeBroadcaster(testinfra.Redis),
		localBroadcasters: localBroadcasters,
		testServer:        testServer,
	}
}

var TestSessionID1 = "testing-session-id-1"
var TestSessionID2 = "testing-session-id-2"
var TestGameID1 = "game1"

func createTestSessions(t *testing.T, redis db.Redis) {
	t.Helper()
	ctx := t.Context()

	for _, session := range []struct {
		ID     string
		Player model.PlayerState
		Expiry time.Duration
	}{
		{ID: TestSessionID1, Player: model.MakePlayer(1, "user1", "us"), Expiry: SessionMaxAge},
		{ID: TestSessionID2, Player: model.MakePlayer(2, "user2", "us"), Expiry: SessionMaxAge},
	} {
		data, err := model.MarshalPlayer(session.Player)
		if err != nil {
			t.Fatalf("marshal session: %v", err)
		}
		sessionKey := fmt.Sprintf("{%s}session/%s", session.ID, session.ID)
		if err := redis.Cache.SetEx(ctx, sessionKey, data, session.Expiry).Err(); err != nil {
			t.Fatalf("set session: %v", err)
		}
	}
}

func createTestChessStates(t *testing.T, redis db.Redis) {
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

	ctx := t.Context()
	for _, state := range testStates {
		gameKey := fmt.Sprintf("game/%s", state.ID)

		bytes, err := proto.Marshal(model.SerializeChessState(state))
		if err != nil {
			t.Fatalf("marshal chess state: %v", err)
		}
		if err := redis.GameStore.Set(ctx, gameKey, bytes, 0).Err(); err != nil {
			t.Fatalf("set chess state: %v", err)
		}
	}
}

type updtLbChangeSet struct {
	Mode    model.GameMode
	ID      int64
	EloDiff float64
}

func createLeaderboard(t *testing.T, rdb db.Redis, changes ...updtLbChangeSet) {
	ctx := t.Context()
	pipe := rdb.Cache.Pipeline()
	for _, change := range changes {
		modeLbZSet := fmt.Sprintf("%s/mode:%s", rdb.LeaderboardZSet, change.Mode.String())
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
