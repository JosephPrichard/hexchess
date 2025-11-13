package web

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type CasterState struct {
	CountsCaster *data.UniCaster
	GamesCaster  *data.MultiCasterMap
	UsersCaster  *data.MultiCasterMap
}

type CountryState struct {
	CountryList []string
	CountryMap  map[string]struct{}
}

type ServerState struct {
	data.Stores
	CasterState
	CountryState
	MakeID func() string
}

func makeCasterState() CasterState {
	return CasterState{
		CountsCaster: data.MakeUniCaster("counts-caster"),
		GamesCaster:  data.MakeMultiCasterMap("games-caster", data.GameExpireDur),
		UsersCaster:  data.MakeMultiCasterMap("users-caster", -1),
	}
}

func makeCountryState(countryList []string) CountryState {
	if countryList == nil {
		countryList = []string{}
	}
	countryMap := make(map[string]struct{})
	for _, c := range countryList {
		countryMap[c] = struct{}{}
	}
	return CountryState{CountryList: countryList, CountryMap: countryMap}
}

func MakeServerState(stores data.Stores, countryList []string) ServerState {
	return ServerState{
		Stores:       stores,
		CasterState:  makeCasterState(),
		CountryState: makeCountryState(countryList),
		MakeID:       func() string { return uuid.NewString() },
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
		handle(method, pattern, makeRestHandler(state, handler))
	}
	handleSse := func(method string, pattern string, handler SseHandler) {
		handle(method, pattern, makeSseHandler(state, handler))
	}

	handleRest("POST", "/api/register", HandleRegister)
	handleRest("POST", "/api/login", HandleLogin)
	handleRest("POST", "/api/session/temp", HandleCreateTempSession)
	handleRest("POST", "/api/session/refresh", HandleRefreshSession)
	handleRest("POST", "/api/logout", HandleLogout)
	handleRest("POST", "/api/users/password", HandleUpdatePassword)
	handleRest("POST", "/api/users", HandleUpdateUser)
	handleRest("POST", "/api/games/create", HandleCreateGame)
	handleRest("POST", "/api/challenges/update", HandleUpdateChallenge)
	handleRest("POST", "/api/challenges/create", HandleCreateChallenge)
	handleRest("POST", "/api/make-move", HandleMakeMove)

	handleRest("GET", "/api/players", HandleGetPlayer)
	handleRest("GET", "/api/players/self", HandleGetSelf)
	handleRest("GET", "/api/players/search", HandleSearchPlayers)
	handleRest("GET", "/api/leaderboard", HandleGetLeaderboard)
	handleRest("GET", "/api/challenges", HandleGetChallenges)
	handleRest("GET", "/api/replays", HandleGetUserReplays)
	handleRest("GET", "/api/chess/rooms", HandleGetChessRoomList)
	handleRest("GET", "/api/replay", HandleGetReplay)
	handleRest("GET", "/api/replay/move-list", HandleGetReplayMoveList)

	handleSse("GET", "/api/events/count", HandleCountEvents)
	handleSse("GET", "/api/events/user", HandleUserEvents)

	handle("GET", "/api/initial-board", makeJsonHandler(chess.InitialBoard()))
	handle("GET", "/api/countries", makeJsonHandler(state.CountryList))

	handle("GET", "/api/ws/game", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "begin game ws connection", "method", r.Method, "url", r.URL)
		HandleGameplayWs(w, r, state)
	}))

	mux.HandleFunc("/api/healthcheck", makeHealthCheck(state))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(NotFoundErrorJSON)
	})

	fmt.Fprintf(util.LogWriter, "%s", sb.String())
	return mux
}

func makeHealthCheck(state ServerState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connPrim := state.Rdb.Primary.Get()
		defer connPrim.Close()

		connPs := state.Rdb.PubSub.Get()
		defer connPs.Close()

		_, primErr := connPrim.Do("PING")
		_, psErr := connPs.Do("PING")
		_, dbErr := state.Pool.Exec(context.Background(), "SELECT 1;")

		failures := make(map[string]string)
		if primErr != nil {
			failures["redisPrimary"] = primErr.Error()
		}
		if psErr != nil {
			failures["redisPubsub"] = psErr.Error()
		}
		if dbErr != nil {
			failures["postgresDB"] = dbErr.Error()
		}

		status := "OK"
		if len(failures) == 3 {
			status = "DOWN"
		} else if len(failures) > 0 {
			status = "PARTIAL_AVAILABILITY"
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
}
