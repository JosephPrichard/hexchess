package network_test

import (
	"context"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/itest"
	"hexchess-svc/matchmaker"
	"hexchess-svc/network"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/rpc"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/goutil"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type serviceMocks struct {
	Entropy    entropy.Generator
	Remote     cloud.SDKs
	Dispatcher goutil.Dispatcher
}

func setupMuxTest(t slogutil.TestLogger, mocks *serviceMocks) (http.Handler, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}

	infra := itest.SetupIntegrationTest(t)

	mux := network.NewServeMux(network.HttpServerConfig{
		// INFRASTRUCTURE
		Database: infra.Database,
		Redis:    infra.Redis,
		AWS:      infra.AWS,
		EsClient: infra.Es,
		// INFRA CLIENTS
		RiverClient: database.NewRiverClient(infra.Pool),
		Broadcaster: publishers.NewBroadcaster(infra.Redis, goutil.SyncDispatcher{}),
		// API MOCKS/STUBS
		SDKs:       mocks.Remote,
		Entropy:    mocks.Entropy,
		Dispatcher: mocks.Dispatcher,
	})
	return mux, infra
}

type websocketTestContext struct {
	testinfra    itest.TestInfra
	broadcasters *subscribers.LocalBroadcasters
	testServer   *httptest.Server
}

func (s *websocketTestContext) getWsURL() string {
	return strings.Replace(s.testServer.URL, "http", "ws", 1)
}

func (s *websocketTestContext) Shutdown() {
	s.testServer.Close() // *MUST* be shutdown before infrastructure/worker pools are shutdown
	s.testinfra.Close()
	s.broadcasters.Shutdown()
}

func setupWebsocketTest(t *testing.T) websocketTestContext {
	infra := itest.SetupIntegrationTest(t)

	localBroadcasters := subscribers.NewLocalBroadcasters(subscribers.DefaultBrokerConfig)
	localBroadcasters.Listen(t.Context(), infra.Redis)

	createTestSessions(t, infra.Redis)
	createTestChessStates(t, infra.Redis)

	testServer := httptest.NewServer(network.NewServeMux(network.HttpServerConfig{
		// INFRASTRUCTURE
		Database: infra.Database,
		Redis:    infra.Redis,
		AWS:      infra.AWS,
		EsClient: infra.Es,
		// INFRA CLIENTS
		RiverClient: database.NewRiverClient(infra.Pool),
		Broadcaster: publishers.NewBroadcaster(infra.Redis, goutil.SyncDispatcher{}),
		// PUB/SUB routing (in memory-impl) required for websockets
		Broadcasters: localBroadcasters,
		// API MOCKS/STUBS
		Entropy:    &entropy.StableSource{},
		Dispatcher: goutil.SyncDispatcher{},
	}))
	return websocketTestContext{testinfra: infra, broadcasters: localBroadcasters, testServer: testServer}
}

type sseTestContext struct {
	testinfra       itest.TestInfra
	broadcasters    *subscribers.LocalBroadcasters
	testBroadcaster publishers.Broadcaster
	testServer      *httptest.Server
}

func (s *sseTestContext) Shutdown() {
	s.testServer.Close() // *MUST* be shutdown before infrastructure/worker pools are shutdown
	s.testinfra.Close()
	s.broadcasters.Shutdown()
}

func setupSSETest(t *testing.T) sseTestContext {
	infra := itest.SetupIntegrationTest(t)

	localBroadcasters := subscribers.NewLocalBroadcasters(subscribers.DefaultBrokerConfig)
	localBroadcasters.Listen(t.Context(), infra.Redis)

	createTestSessions(t, infra.Redis)

	testServer := httptest.NewServer(network.NewServeMux(network.HttpServerConfig{
		// INFRASTRUCTURE
		Database: infra.Database,
		Redis:    infra.Redis,
		AWS:      infra.AWS,
		EsClient: infra.Es,
		// INFRA CLIENTS
		RiverClient: database.NewRiverClient(infra.Pool),
		Broadcaster: publishers.NewBroadcaster(infra.Redis, goutil.SyncDispatcher{}),
		// PUB/SUB routing (in memory-impl) required for SSE
		Broadcasters: localBroadcasters,
		// API MOCKS/STUBS
		Entropy:    &entropy.StableSource{},
		Dispatcher: goutil.SyncDispatcher{},
	}))
	return sseTestContext{
		testinfra:       infra,
		testBroadcaster: publishers.NewBroadcaster(infra.Redis, goutil.SyncDispatcher{}),
		broadcasters:    localBroadcasters,
		testServer:      testServer,
	}
}

type grpcTestContext struct {
	server *grpc.Server
	client *grpc.ClientConn
}

type grpcTestConfig struct {
	matchmakingServerConfig matchmaker.MatchmakerServiceConfig
}

const grpcServerBufSize = 1024 * 1024

func setupGRPCTest(t *testing.T, cfg grpcTestConfig) grpcTestContext {
	listener := bufconn.Listen(grpcServerBufSize)
	server := grpc.NewServer()

	matchmakerServer := network.NewGRPCServer(matchmaker.NewMatchmakerService(cfg.matchmakingServerConfig))
	rpc.RegisterMatchmakerServiceServer(server, &matchmakerServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			slog.Info("grpc server exited", "status", err)
		}
	}()

	client, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, address string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client did not connect: %v", err)
	}

	return grpcTestContext{server: server, client: client}
}

func (s *grpcTestContext) Shutdown() {
	s.server.Stop()
	s.client.Close()
}
