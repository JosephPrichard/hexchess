package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"hexchess-svc/service"
	"hexchess-svc/util/logutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/hellofresh/health-go/v5"
	pgHealth "github.com/hellofresh/health-go/v5/checks/postgres"
	redisHealth "github.com/hellofresh/health-go/v5/checks/redis"
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-trace")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type Setup struct {
	Services       *svc.HexchessServices
	Broadcasers    *svc.LocalBroadcasters
	EntropySource  svc.EntropySource
	AllowedOrigins string
}

type StaticData struct {
	validCountries map[string]bool
	countryList    []string
}

type API struct {
	services      *svc.HexchessServices
	broadcasters  *svc.LocalBroadcasters
	entropy       svc.EntropySource
	authenticator Authenticator
	staticData    StaticData
}

func MakeStaticData() StaticData {
	var countryList []string
	if err := json.Unmarshal(assets.CountryListJson, &countryList); err != nil {
		logutil.FatalErr("unmarshal country list", err)
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

func MakeServeMux(setup Setup) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(setup.AllowedOrigins))

	if setup.EntropySource == nil {
		setup.EntropySource = &svc.RealEntropySource{}
	}
	server := API{
		services:      setup.Services,
		broadcasters:  setup.Broadcasers,
		entropy:       setup.EntropySource,
		authenticator: Authenticator{services: setup.Services},
		staticData:    MakeStaticData(),
	}

	r.Post("/api/register", Rest(server.HandleRegister))
	r.Post("/api/login", Rest(server.HandleLogin))
	r.Post("/api/login/google", Rest(server.HandleGoogleLogin))
	r.Post("/api/session/temp", Rest(server.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(server.HandleRefreshSession))
	r.Post("/api/logout", Rest(server.HandleLogout))
	r.Post("/api/users/password", Rest(server.HandleUpdatePassword))
	r.Post("/api/users", Rest(server.HandleUpdateUser))
	r.Post("/api/games/create", Rest(server.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(server.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(server.HandleCreateChallenge))
	r.Post("/api/users/profile-pics", Rest(server.HandleUploadProfilePic))

	r.Get("/api/players", Rest(server.HandleGetPlayer))
	r.Get("/api/players/self", Rest(server.HandleGetSelf))
	r.Get("/api/players/search", Rest(server.HandleSearchPlayers))
	r.Get("/api/leaderboard", Rest(server.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(server.HandleGetChallenges))
	r.Get("/api/replays", Rest(server.HandleGetUserReplays))
	r.Get("/api/replay", Rest(server.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(server.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(server.HandleGetMoveReplay))
	r.Get("/api/game/rooms", Rest(server.HandleGetChessMetas))
	r.Get("/api/game/rooms/chats", Rest(server.HandleGetGameChats))
	r.Get("/api/game/rooms/exists", Rest(server.HandleGameExistence))
	r.Get("/api/users/profile-pics", Rest(server.HandleGetProfilePic))

	r.Get("/api/events/count", SSE(server.HandleCountEvents))
	r.Get("/api/events/user", SSE(server.HandleUserEvents))
	r.Get("/api/events/active", SSE(server.HandleActiveConn))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(server.staticData.countryList))

	r.Get("/api/ws/game", server.HandleGameWs)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		writeJSON(w, http.StatusNotFound, ServiceView{Status: http.StatusNotFound, Message: "ROUTE_NOT_FOUND"})
	})

	var strs []string
	_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		strs = append(strs, fmt.Sprintf("%s %s", method, route))
		return nil
	})
	fmt.Fprintf(logutil.LogWriter, "%s\n", strings.Join(strs, "\n"))

	return r
}

type HealthCheckConfig struct {
	PostgresDSN     string
	RedisPrimaryDSN string
	RedisPubSubDSN  string
}

func WithHealthCheck(mux *chi.Mux, config HealthCheckConfig) {
	h, err := health.New(
		health.WithComponent(
			health.Component{Name: "hexchess-svc", Version: "v1.0"},
		),
		health.WithChecks(
			health.Config{
				Name:    "Postgres",
				Timeout: time.Second * 1,

				Check: pgHealth.New(pgHealth.Config{
					DSN: config.PostgresDSN,
				}),
			},
			health.Config{
				Name:    "RedisPrimary",
				Timeout: time.Second * 1,
				Check: redisHealth.New(redisHealth.Config{
					DSN: config.RedisPrimaryDSN,
				}),
			},
			health.Config{
				Name:      "RedisPubsub",
				Timeout:   time.Second * 1,
				SkipOnErr: true,
				Check: redisHealth.New(redisHealth.Config{
					DSN: config.RedisPubSubDSN,
				}),
			},
		),
	)
	if err != nil {
		logutil.FatalErr("failed to create health checker", err)
	}
	mux.Get("/healthcheck", h.HandlerFunc)
}
