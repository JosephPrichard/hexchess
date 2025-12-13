package web

import (
	"context"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/infra"
	"hexchess-svc/svc"
	"hexchess-svc/util"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CasterState struct {
	CountsCaster *svc.UniCaster
	GamesCaster  *svc.MultiCasterMap
	UsersCaster  *svc.MultiCasterMap
}

type CountryState struct {
	CountryList []string
	CountryMap  map[string]struct{}
}

type APIKeys struct {
	GoogleAPIKey string
}

type ServerState struct {
	infra.Databases
	CasterState
	CountryState
	APIKeys
	Generators
}

func MakeServerState(databases infra.Databases, countryList []string, googleAPIKey string) ServerState {
	if countryList == nil {
		countryList = []string{}
	}
	countryMap := make(map[string]struct{})
	for _, c := range countryList {
		countryMap[c] = struct{}{}
	}

	return ServerState{
		Databases: databases,
		CasterState: CasterState{
			CountsCaster: svc.MakeUniCaster("counts-caster"),
			GamesCaster:  svc.MakeMultiCasterMap("games-caster", svc.GameExpireDur),
			UsersCaster:  svc.MakeMultiCasterMap("users-caster", -1),
		},
		CountryState: CountryState{
			CountryList: countryList,
			CountryMap:  countryMap,
		},
		APIKeys: APIKeys{
			GoogleAPIKey: googleAPIKey,
		},
		Generators: &RandGenerator{},
	}
}

func withTrace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := r.Header.Get("X-trace")
		if trace == "" {
			trace = uuid.NewString()
		}
		r = r.WithContext(context.WithValue(r.Context(), util.Trace, trace))
		next.ServeHTTP(w, r)
	})
}

func withCors(next http.Handler, allowedOrigins string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-trace")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight (OPTIONS)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func HandleRoot(state ServerState, allowedOrigins string) http.Handler {
	mux := http.NewServeMux()

	var sb strings.Builder
	sb.WriteString("starting server with registered routes:\n")

	handle := func(method string, pattern string, handler http.Handler) {
		handler = withTrace(withCors(handler, allowedOrigins))
		mux.Handle(method+" "+pattern, handler)
		mux.Handle("OPTIONS "+pattern, handler)
		sb.WriteString(fmt.Sprintf("\t%s\n", pattern))
	}
	handleRest := func(method string, pattern string, handler RestHandler) {
		handle(method, pattern, makeRestHandler(&state, handler))
	}
	handleSse := func(method string, pattern string, handler SseHandler) {
		handle(method, pattern, makeSseHandler(&state, handler))
	}

	handleRest("POST", "/api/register", HandleRegister)
	handleRest("POST", "/api/login", HandleLogin)
	handleRest("POST", "/api/login/google", HandleGoogleLogin)
	handleRest("POST", "/api/session/temp", HandleCreateTempSession)
	handleRest("POST", "/api/session/refresh", HandleRefreshSession)
	handleRest("POST", "/api/logout", HandleLogout)
	handleRest("POST", "/api/users/password", HandleUpdatePassword)
	handleRest("POST", "/api/users", HandleUpdateUser)
	handleRest("POST", "/api/games/create", HandleCreateGame)
	handleRest("POST", "/api/challenges/update", HandleUpdateChallenge)
	handleRest("POST", "/api/challenges/create", HandleCreateChallenge)

	handleRest("GET", "/api/players", HandleGetPlayer)
	handleRest("GET", "/api/players/self", HandleGetSelf)
	handleRest("GET", "/api/players/search", HandleSearchPlayers)
	handleRest("GET", "/api/leaderboard", HandleGetLeaderboard)
	handleRest("GET", "/api/challenges", HandleGetChallenges)
	handleRest("GET", "/api/replays", HandleGetUserReplays)
	handleRest("GET", "/api/chess/rooms", HandleGetChessRoomList)
	handleRest("GET", "/api/replay", HandleGetReplay)
	handleRest("GET", "/api/replay/elo-history", HandleGetEloHistories)
	handleRest("GET", "/api/replay/move-list", HandleGetReplayMoveList)

	handleSse("GET", "/api/events/count", HandleCountEvents)
	handleSse("GET", "/api/events/user", HandleUserEvents)

	handle("GET", "/api/initial-board", makeJsonHandler(chess.InitialBoard()))
	handle("GET", "/api/countries", makeJsonHandler(state.CountryList))

	handle("GET", "/api/ws/game", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "begin game ws connection", "method", r.Method, "url", r.URL)
		HandleGameWs(w, r, state)
	}))

	handleRest("GET", "/api/healthcheck", HandleHealthCheck)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		writeJSON(w, http.StatusNotFound, ServiceView{Status: http.StatusNotFound, Message: "ROUTE_NOT_FOUND"})
	})

	fmt.Fprintf(util.LogWriter, "%s", sb.String())
	return mux
}

func HandleHealthCheck(state *ServerState, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	healthChecks := []struct {
		Name  string
		Check func() error
	}{
		{
			Name: "redisPrimary",
			Check: func() error {
				_, err := state.Rdb.Cache.Ping(ctx).Result()
				return err
			},
		},
		{
			Name: "redisPubsub",
			Check: func() error {
				_, err := state.Rdb.PubSub.Ping(ctx).Result()
				return err
			},
		},
		{
			Name: "postgresDB",
			Check: func() error {
				_, err := state.Pdb.GetPool().Exec(ctx, "SELECT 1;")
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

	return nil
}
