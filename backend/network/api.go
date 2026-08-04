package network

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"log/slog"
	"net/http"
	"time"

	svc "hexchess-svc/service"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/logutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/hellofresh/health-go/v5"
)

func RouteMiddleware(allowedOrigins string) func(handlerFunc http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace := r.Header.Get("X-trace")
			if trace == "" {
				trace = uuid.NewString()
			}
			r = r.WithContext(context.WithValue(r.Context(), logutil.Trace, trace))

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-trace, Content-Digest, Rollout")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type ServerSetup struct {
	Services       *svc.HexchessServices
	Broadcasters   *pubsub.LocalBroadcasters
	Broadcaster    pubsub.Broadcaster
	EntropySource  entropy.Generator
	Dispatcher     async.Dispatcher
	AllowedOrigins string
}

type StaticData struct {
	validCountries map[string]bool
	countryList    []string
}

type API struct {
	services      *svc.HexchessServices
	broadcasters  *pubsub.LocalBroadcasters
	broadcaster   pubsub.Broadcaster
	dispatcher    async.Dispatcher
	entropy       entropy.Generator
	authenticator Authenticator
	staticData    StaticData
}

func NewStaticData() StaticData {
	var countryList []string
	if err := json.Unmarshal(assets.CountryListJson, &countryList); err != nil {
		logutil.Fatal("unmarshal country list", err)
	}
	if countryList == nil {
		countryList = []string{}
	}
	validCountries := make(map[string]bool)
	for _, country := range countryList {
		validCountries[country] = true
	}
	return StaticData{validCountries: validCountries, countryList: countryList}
}

func NewServeMux(setup ServerSetup, opts ...func(*chi.Mux)) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(setup.AllowedOrigins))

	if setup.EntropySource == nil {
		setup.EntropySource = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}
	server := API{
		services:      setup.Services,
		broadcaster:   setup.Broadcaster,
		broadcasters:  setup.Broadcasters,
		dispatcher:    setup.Dispatcher,
		entropy:       setup.EntropySource,
		authenticator: Authenticator{services: setup.Services},
		staticData:    NewStaticData(),
	}

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
