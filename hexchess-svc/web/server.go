package web

import (
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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

type State struct {
	Databases      db.Databases
	Broadcasters   svc.Broadcasters
	Generators     outbound.Generators
	OutboundAPIs   outbound.RemoteAPIs
	CountryList    []string
	AllowedOrigins string
}

func HandleRoot(state State) http.Handler {
	countryList := state.CountryList
	if countryList == nil {
		countryList = []string{}
	}
	validCountries := make(map[string]bool)
	for _, country := range state.CountryList {
		validCountries[country] = true
	}

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(state.AllowedOrigins))

	rest := RestHandler{state.Databases, state.Generators, state.OutboundAPIs, validCountries}
	sse := SSEHandler{state.Databases.Rdb, state.Generators, state.Broadcasters}
	gameplay := GameplayHandler{state.Databases, state.Broadcasters, state.Generators}
	healthcheck := HealthCheckHandler{state.Databases}

	r.Post("/api/register", Rest(rest.HandleRegister))
	r.Post("/api/login", Rest(rest.HandleLogin))
	r.Post("/api/login/google", Rest(rest.HandleGoogleLogin))
	r.Post("/api/session/temp", Rest(rest.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(rest.HandleRefreshSession))
	r.Post("/api/logout", Rest(rest.HandleLogout))
	r.Post("/api/users/password", Rest(rest.HandleUpdatePassword))
	r.Post("/api/users", Rest(rest.HandleUpdateUser))
	r.Post("/api/games/create", Rest(rest.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(rest.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(rest.HandleCreateChallenge))

	r.Get("/api/players", Rest(rest.HandleGetPlayer))
	r.Get("/api/players/self", Rest(rest.HandleGetSelf))
	r.Get("/api/players/search", Rest(rest.HandleSearchPlayers))
	r.Get("/api/leaderboard", Rest(rest.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(rest.HandleGetChallenges))
	r.Get("/api/replays", Rest(rest.HandleGetUserReplays))
	r.Get("/api/chess/rooms", Rest(rest.HandleGetChessRoomList))
	r.Get("/api/replay", Rest(rest.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(rest.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(rest.HandleGetReplayMoveList))

	r.Get("/api/events/count", SSE(sse.HandleCountEvents))
	r.Get("/api/events/user", SSE(sse.HandleUserEvents))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(countryList))

	r.Get("/api/ws/game", gameplay.HandleGameWs)

	r.Get("/api/healthcheck", healthcheck.HandleHealthCheck)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		writeJSON(w, http.StatusNotFound, ServiceView{Status: http.StatusNotFound, Message: "ROUTE_NOT_FOUND"})
	})

	var sb strings.Builder
	_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		sb.WriteString(fmt.Sprintf("%s %s\n", method, route))
		return nil
	})
	fmt.Println(sb.String())

	return r
}

type HealthCheckHandler struct {
	db.Databases
}

func (h *HealthCheckHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	healthChecks := []struct {
		Name  string
		Check func() error
	}{
		{
			Name: "redisPrimary",
			Check: func() error {
				_, err := h.Rdb.Cache.Ping(ctx).Result()
				return err
			},
		},
		{
			Name: "redisPubsub",
			Check: func() error {
				conn := h.Rdb.PubSub.Get()
				defer conn.Close()
				_, err := conn.Do("PING")
				return err
			},
		},
		{
			Name: "postgresDB",
			Check: func() error {
				_, err := h.Pdb.Pool.Exec(ctx, "SELECT 1;")
				return err
			},
		},
	}

	failures := make(map[string]string)
	for h := range healthChecks {
		if err := healthChecks[h].Check(); err != nil {
			failures["redisPrimary"] = err.Error()
		}
	}
	status := "OK"
	if len(failures) == len(healthChecks) {
		status = "DOWN"
	} else if len(failures) > 0 {
		status = "PARTIAL"
	}

	writeJSON(w, http.StatusOK, struct {
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
		Failures  map[string]string `json:"failures"`
	}{
		Status:    status,
		Timestamp: time.Now(),
		Failures:  failures,
	})
}
