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
)

type ServerState struct {
	data.Stores
	CountsCaster *data.UniCaster
	GamesCaster  *data.MultiCasterMap
	UsersCaster  *data.MultiCasterMap
	CountryList  []string
	CountryMap   map[string]struct{}
	MakeID       func() string // mock uniquely generated request IDs in tests
}

func MakeServerState(stores data.Stores, countryList []string) ServerState {
	if countryList == nil {
		countryList = []string{}
	}
	countryMap := make(map[string]struct{})
	for _, c := range countryList {
		countryMap[c] = struct{}{}
	}
	return ServerState{
		Stores:       stores,
		CountsCaster: data.MakeUniCaster("counts-caster"),
		GamesCaster:  data.MakeMultiCasterMap("games-caster", data.GameExpireDur),
		UsersCaster:  data.MakeMultiCasterMap("users-caster", -1),
		CountryList:  countryList,
		CountryMap:   countryMap,
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(NotFoundErrorJSON)
	})

	fmt.Fprintf(util.LogWriter, "%s", sb.String())
	return mux
}
