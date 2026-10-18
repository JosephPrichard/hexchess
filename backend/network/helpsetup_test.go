package network_test

import (
	"context"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/network"
	"hexchess-svc/pubsub"
	"hexchess-svc/rpc"
	"hexchess-svc/service"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
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
	Dispatcher async.Dispatcher
}

func setupMuxTest(t slogutil.TestLogger, mocks *serviceMocks) (http.Handler, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}

	infra := itest.SetupIntegrationTest(t)

	mux := network.NewServeMux(network.HttpServerConfig{
		Database:    infra.Database,
		Redis:       infra.Redis,
		AWS:         infra.AWS,
		SDKs:        mocks.Remote,
		Entropy:     mocks.Entropy,
		Dispatcher:  mocks.Dispatcher,
		Broadcaster: pubsub.NewSyncBroadcaster(infra.Redis),
	})
	return mux, infra
}

type websocketTestContext struct {
	testinfra    itest.TestInfra
	broadcasters *pubsub.LocalBroadcasters
	testServer   *httptest.Server
}

func (s *websocketTestContext) getWsURL() string {
	return strings.Replace(s.testServer.URL, "http", "ws", 1)
}

func (s *websocketTestContext) Shutdown() {
	s.testinfra.Close()
	s.testServer.Close()
}

func setupWebsocketTest(t *testing.T) websocketTestContext {
	testinfra := itest.SetupIntegrationTest(t)

	localBroadcasters := pubsub.NewLocalBroadcasters()
	<-localBroadcasters.ListenGameMessages(testinfra.Redis)

	createTestSessions(t, testinfra.Redis)
	createTestChessStates(t, testinfra.Redis)

	testServer := httptest.NewServer(network.NewServeMux(network.HttpServerConfig{
		Database:     testinfra.Database,
		Redis:        testinfra.Redis,
		Broadcasters: localBroadcasters,
		Broadcaster:  pubsub.NewSyncBroadcaster(testinfra.Redis),
		Dispatcher:   async.SyncDispatcher{},
	}))
	return websocketTestContext{testinfra: testinfra, broadcasters: localBroadcasters, testServer: testServer}
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

	testServer := httptest.NewServer(network.NewServeMux(network.HttpServerConfig{
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

type grpcTestContext struct {
	server *grpc.Server
	client *grpc.ClientConn
}

type grpcTestConfig struct {
	matchmakingServerConfig *service.MatchmakerServiceConfig
}

const grpcServerBufSize = 1024 * 1024

func setupGRPCTest(t *testing.T, cfg grpcTestConfig) grpcTestContext {
	listener := bufconn.Listen(grpcServerBufSize)
	server := grpc.NewServer()

	matchmakerServer := network.NewGRPCServer(service.NewMatchmakerService(cfg.matchmakingServerConfig))
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

	return grpcTestContext{
		server: server,
		client: client,
	}
}

func (s *grpcTestContext) Shutdown() {
	s.server.Stop()
	s.client.Close()
}
