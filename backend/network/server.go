package network

import (
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	sessionSvc "hexchess-svc/service/session"
	"hexchess-svc/utils/entropy"
	"log/slog"
	"net/http"
	"time"

	"hexchess-svc/utils/async"
	"hexchess-svc/utils/logutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hellofresh/health-go/v5"
)

type ServeMuxSetup struct {
	Database       database.Database
	RiverClient    producers.RiverClientAPI
	Redis          cache.Redis
	Broadcaster    pubsub.Broadcaster
	AWS            cloud.AWSClient
	SDKs           cloud.SDKs
	Entropy        entropy.Generator
	Dispatcher     async.Dispatcher
	Broadcasters   *pubsub.LocalBroadcasters
	AllowedOrigins string
}

type Server struct {
	services      Services
	broadcasters  *pubsub.LocalBroadcasters
	broadcaster   pubsub.Broadcaster
	dispatcher    async.Dispatcher
	entropy       entropy.Generator
	authenticator Authenticator
	staticData    StaticData
}

func NewServer(setup ServeMuxSetup) Server {
	return Server{
		services: NewAPIServices(SetupAPIServices{
			Database:    setup.Database,
			RiverClient: setup.RiverClient,
			Redis:       setup.Redis,
			Broadcaster: setup.Broadcaster,
			AWS:         setup.AWS,
			SDKs:        setup.SDKs,
			Entropy:     setup.Entropy,
			Dispatcher:  setup.Dispatcher,
		}),
		broadcaster:   setup.Broadcaster,
		broadcasters:  setup.Broadcasters,
		dispatcher:    setup.Dispatcher,
		entropy:       setup.Entropy,
		authenticator: Authenticator{services: sessionSvc.NewSessionService(setup.Redis)},
		staticData:    NewStaticData(),
	}
}

func NewServeMux(setup ServeMuxSetup, opts ...func(*chi.Mux)) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(setup.AllowedOrigins))

	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	server := NewServer(ServeMuxSetup{
		Database:       setup.Database,
		RiverClient:    setup.RiverClient,
		Redis:          setup.Redis,
		Broadcaster:    setup.Broadcaster,
		AWS:            setup.AWS,
		SDKs:           setup.SDKs,
		Entropy:        setup.Entropy,
		Dispatcher:     setup.Dispatcher,
		Broadcasters:   setup.Broadcasters,
		AllowedOrigins: setup.AllowedOrigins,
	})

	r.Post("/api/register", Rest(server.HandleRegister))
	r.Post("/api/login", Rest(server.HandleLogin))
	r.Post("/api/login/google", Rest(server.HandleGoogleLogin))
	r.Post("/api/logout", Rest(server.HandleLogout))
	r.Post("/api/session/temp", Rest(server.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(server.HandleRefreshSession))
	r.Post("/api/users/password", Rest(server.HandleUpdatePassword))
	r.Post("/api/users", Rest(server.HandleUpdateUser))
	r.Post("/api/games/create", Rest(server.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(server.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(server.HandleCreateChallenge))
	r.Post("/api/users/profile-pics", Rest(server.HandleUploadProfilePic))

	r.Get("/api/players", Rest(server.HandleGetPlayer))
	r.Get("/api/players/self", Rest(server.HandleGetSelf))
	r.Get("/api/players/search", Rest(server.HandleSearchPlayers))
	r.Get("/api/players/activity", Rest(server.HandleUserActivityCheck))
	r.Get("/api/leaderboard", Rest(server.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(server.HandleGetChallenges))
	r.Get("/api/challenges/count", Rest(server.HandleCountUserChallenges))
	r.Get("/api/replays", Rest(server.HandleSearchReplays))
	r.Get("/api/replay", Rest(server.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(server.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(server.HandleGetMoveReplay))
	r.Get("/api/game/rooms", Rest(server.HandleGetGameMetadata))
	r.Get("/api/game/rooms/chats", Rest(server.HandleGetGameChats))
	r.Get("/api/game/rooms/exists", Rest(server.HandleGameExistence))
	r.Get("/api/users/profile-pics", Rest(server.HandleGetProfilePic))
	r.Get("/api/tournament", Rest(server.HandleGetTournament))
	r.Get("/api/tournaments", Rest(server.HandleGetTournaments))

	r.Get("/api/events/count", SSE(server.HandleCountEvents))
	r.Get("/api/events/user", SSE(server.HandleUserEvents))
	r.Get("/api/events/active", SSE(server.HandleActiveConn))
	r.Get("/api/events/tournament", SSE(server.HandleTournamentEvents))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(server.staticData.countryList))

	r.Get("/api/ws/game", server.HandleGameWs)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL.String())
		writeJSON(w, http.StatusNotFound, ServiceResp{Status: http.StatusNotFound, Message: "ROUTE_NOT_FOUND"})
	})

	for _, opt := range opts {
		opt(r)
	}

	var handlers []string
	_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		handlers = append(handlers, fmt.Sprintf("%s %s", method, route))
		return nil
	})
	slog.Info("initialized serve mux", "handlers", handlers)

	return r
}

type HealthConfig struct {
	PostgresCheck     health.CheckFunc
	PostgresReadCheck health.CheckFunc
	RedisPrimaryCheck health.CheckFunc
	RedisPubSubCheck  health.CheckFunc
}

const (
	PostgresLabel     = "Postgres"
	PostgresReadLabel = "ReadPostgres"
	RedisPubsubLabel  = "RedisPubsub"
	RedisPrimaryLabel = "RedisPrimary"
)

func NewHealthCheck(config HealthConfig) func(*chi.Mux) {
	return func(mux *chi.Mux) {
		healthChecks := []health.Config{
			{
				Name:    PostgresLabel,
				Timeout: time.Second * 5,
				Check:   config.PostgresCheck,
			},
			{
				Name:    PostgresReadLabel,
				Timeout: time.Second * 5,
				Check:   config.PostgresReadCheck,
			},
			{
				Name:      RedisPubsubLabel,
				Timeout:   time.Second * 5,
				SkipOnErr: true,
				Check:     config.RedisPubSubCheck,
			},
			{
				Name:      RedisPrimaryLabel,
				Timeout:   time.Second * 5,
				SkipOnErr: true,
				Check:     config.RedisPrimaryCheck,
			},
		}

		healthcheckHandler, err := health.New(
			health.WithComponent(health.Component{Name: "hexchess-svc", Version: "v1.0"}),
			health.WithChecks(healthChecks...),
		)
		if err != nil {
			logutil.Fatal("failed to create health check handler", err)
		}

		mux.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
			//slog.InfoContext(r.Context(), "health check", "method", r.Method, "url", r.URL.String())
			healthcheckHandler.HandlerFunc(w, r)
		})
	}
}
