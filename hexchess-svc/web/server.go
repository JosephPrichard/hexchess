package web

import (
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Broadcasters struct {
	CountsCaster *svc.UniCaster
	GamesCaster  *svc.MultiCasterMap
	UsersCaster  *svc.MultiCasterMap
}

type CountryRegistry struct {
	CountryList    []string
	ValidCountries map[string]bool
}

type ServerState struct {
	// data
	CountryRegistry
	// infra
	db.Databases
	Broadcasters
	// interfaces
	Generators
	outbound.APIs
}

type APIKeys struct {
	GoogleAPIKey string
}

type ServerSetup struct {
	Databases    db.Databases
	CountryList  []string
	APIKeys      APIKeys
	Generators   Generators
	OutboundAPIs outbound.APIs
}

func MakeServerState(setup ServerSetup) *ServerState {
	// default initialize setup data, used for tests
	if setup.CountryList == nil {
		setup.CountryList = []string{}
	}
	validCountries := make(map[string]bool)
	for _, country := range setup.CountryList {
		validCountries[country] = true
	}
	// server state
	return &ServerState{
		// data
		CountryRegistry: CountryRegistry{CountryList: setup.CountryList, ValidCountries: validCountries},
		// infra
		Databases: setup.Databases,
		Broadcasters: Broadcasters{
			CountsCaster: svc.MakeUniCaster("counts-caster"),
			GamesCaster:  svc.MakeMultiCasterMap("games-caster", svc.GameExpireDur),
			UsersCaster:  svc.MakeMultiCasterMap("users-caster", -1),
		},
		// interfaces
		Generators: setup.Generators,
		APIs:       setup.OutboundAPIs,
	}
}

func RouteMiddleware(allowedOrigins string) func(handlerFunc http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace := r.Header.Get("X-trace")
			if trace == "" {
				trace = uuid.NewString()
			}
			r = r.WithContext(context.WithValue(r.Context(), util.Trace, trace))

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

func HandleRoot(state *ServerState, allowedOrigins string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(allowedOrigins))

	restApi := RestApi{ServerState: state}
	sseApi := SSEApi{ServerState: state}
	gameplayApi := GameplayApi{ServerState: state}
	healthcheckApi := HealthCheckApi{ServerState: state}

	r.Post("/api/register", Rest(restApi.HandleRegister))
	r.Post("/api/login", Rest(restApi.HandleLogin))
	r.Post("/api/login/google", Rest(restApi.HandleGoogleLogin))
	r.Post("/api/session/temp", Rest(restApi.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(restApi.HandleRefreshSession))
	r.Post("/api/logout", Rest(restApi.HandleLogout))
	r.Post("/api/users/password", Rest(restApi.HandleUpdatePassword))
	r.Post("/api/users", Rest(restApi.HandleUpdateUser))
	r.Post("/api/games/create", Rest(restApi.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(restApi.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(restApi.HandleCreateChallenge))

	r.Get("/api/players", Rest(restApi.HandleGetPlayer))
	r.Get("/api/players/self", Rest(restApi.HandleGetSelf))
	r.Get("/api/players/search", Rest(restApi.HandleSearchPlayers))
	r.Get("/api/leaderboard", Rest(restApi.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(restApi.HandleGetChallenges))
	r.Get("/api/replays", Rest(restApi.HandleGetUserReplays))
	r.Get("/api/chess/rooms", Rest(restApi.HandleGetChessRoomList))
	r.Get("/api/replay", Rest(restApi.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(restApi.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(restApi.HandleGetReplayMoveList))

	r.Get("/api/events/count", SSE(sseApi.HandleCountEvents))
	r.Get("/api/events/user", SSE(sseApi.HandleUserEvents))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(state.CountryList))

	r.Get("/api/ws/game", gameplayApi.HandleGameWs)

	r.Get("/api/healthcheck", healthcheckApi.HandleHealthCheck)

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

type HealthCheckApi struct {
	*ServerState
}

func (api *HealthCheckApi) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	healthChecks := []struct {
		Name  string
		Check func() error
	}{
		{
			Name: "redisPrimary",
			Check: func() error {
				_, err := api.Rdb.Cache.Ping(ctx).Result()
				return err
			},
		},
		{
			Name: "redisPubsub",
			Check: func() error {
				conn := api.Rdb.PubSub.Get()
				defer conn.Close()
				_, err := conn.Do("PING")
				return err
			},
		},
		{
			Name: "postgresDB",
			Check: func() error {
				_, err := api.Pdb.GetPool().Exec(ctx, "SELECT 1;")
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
