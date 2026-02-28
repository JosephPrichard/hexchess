package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"hexchess-svc/chess"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

type Setup struct {
	Services       svc.Services
	CountryList    []string
	AllowedOrigins string
}

type App struct {
	svc.Services
	ValidCountries map[string]bool
}

func MakeRoot(setup Setup) http.Handler {
	countryList := setup.CountryList
	if countryList == nil {
		countryList = []string{}
	}
	validCountries := make(map[string]bool)
	for _, country := range setup.CountryList {
		validCountries[country] = true
	}

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(setup.AllowedOrigins))

	app := App{setup.Services, validCountries}

	r.Post("/api/register", Rest(app.HandleRegister))
	r.Post("/api/login", Rest(app.HandleLogin))
	r.Post("/api/login/google", Rest(app.HandleGoogleLogin))
	r.Post("/api/session/temp", Rest(app.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(app.HandleRefreshSession))
	r.Post("/api/logout", Rest(app.HandleLogout))
	r.Post("/api/users/password", Rest(app.HandleUpdatePassword))
	r.Post("/api/users", Rest(app.HandleUpdateUser))
	r.Post("/api/games/create", Rest(app.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(app.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(app.HandleCreateChallenge))
	r.Post("/api/users/profile-pics", Rest(app.HandleUploadProfilePic))

	r.Get("/api/players", Rest(app.HandleGetPlayer))
	r.Get("/api/players/self", Rest(app.HandleGetSelf))
	r.Get("/api/players/search", Rest(app.HandleSearchPlayers))
	r.Get("/api/leaderboard", Rest(app.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(app.HandleGetChallenges))
	r.Get("/api/replays", Rest(app.HandleGetUserReplays))
	r.Get("/api/chess/rooms", Rest(app.HandleGetChessMetas))
	r.Get("/api/replay", Rest(app.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(app.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(app.HandleGetMoveReplay))
	r.Get("/api/game/exists", Rest(app.HandleGameExistence))
	r.Get("/api/users/profile-pics", Rest(app.HandleGetProfilePic))

	r.Get("/api/events/count", SSE(app.HandleCountEvents))
	r.Get("/api/events/user", SSE(app.HandleUserEvents))
	r.Get("/api/events/active", SSE(app.HandleActiveConn))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(countryList))

	r.Get("/api/ws/game", app.HandleGameWs)

	r.Get("/api/healthcheck", app.HandleHealthCheck)

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

func (app *App) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	healthChecks := []struct {
		Name  string
		Check func() error
	}{
		{
			Name: "rdbPrimary",
			Check: func() error {
				_, err := app.Redis.Cache.Ping(ctx).Result()
				return err
			},
		},
		{
			Name: "rdbPubsub",
			Check: func() error {
				conn := app.Redis.PubSub.Get()
				defer conn.Close()
				_, err := conn.Do("PING")
				return err
			},
		},
		{
			Name: "postgresDB",
			Check: func() error {
				return app.Postgres.HealthCheck(ctx)
			},
		},
	}

	failures := make(map[string]string)
	for _, h := range healthChecks {
		if err := h.Check(); err != nil {
			failures["rdbPrimary"] = err.Error()
		}
	}
	status := "OK"
	if len(failures) > 0 {
		status = "DOWN"
	}
	httpStatus := http.StatusOK
	if status == "DOWN" {
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, struct {
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
		Failures  map[string]string `json:"failures"`
	}{
		Status:    status,
		Timestamp: time.Now(),
		Failures:  failures,
	})
}
