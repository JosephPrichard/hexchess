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

	handle("POST", "/api/register", makeRestHandler(state, HandleRegister))
	handle("POST", "/api/login", makeRestHandler(state, HandleLogin))
	handle("POST", "/api/session/temp", makeRestHandler(state, HandleCreateTempSession))
	handle("POST", "/api/session/refresh", makeRestHandler(state, HandleRefreshSession))
	handle("POST", "/api/logout", makeRestHandler(state, HandleLogout))
	handle("POST", "/api/users/password", makeRestHandler(state, HandleUpdatePassword))
	handle("POST", "/api/users", makeRestHandler(state, HandleUpdateUser))
	handle("POST", "/api/games/create", makeRestHandler(state, HandleCreateGame))
	handle("POST", "/api/challenges/update", makeRestHandler(state, HandleUpdateChallenge))
	handle("POST", "/api/challenges/create", makeRestHandler(state, HandleCreateChallenge))

	handle("GET", "/api/players", makeRestHandler(state, HandleGetPlayer))
	handle("GET", "/api/players/self", makeRestHandler(state, HandleGetSelf))
	handle("GET", "/api/players/search", makeRestHandler(state, HandleSearchPlayers))
	handle("GET", "/api/leaderboard", makeRestHandler(state, HandleGetLeaderboard))
	handle("GET", "/api/challenges", makeRestHandler(state, HandleGetChallenges))
	handle("GET", "/api/replays", makeRestHandler(state, HandleGetUserReplays))
	handle("GET", "/api/chess/rooms", makeRestHandler(state, HandleGetChessRoomList))
	handle("GET", "/api/replay", makeRestHandler(state, HandleGetReplay))
	handle("GET", "/api/replay/move-list", makeRestHandler(state, HandleGetReplayMoveList))

	handle("GET", "/api/events/count", makeSseHandler(state, HandleCountEvents))
	handle("GET", "/api/events/user", makeSseHandler(state, HandleUserEvents))

	handle("GET", "/api/initial-board", makeJsonHandler(chess.InitialBoard()))
	handle("GET", "/api/countries", makeJsonHandler(state.CountryList))

	handle("GET", "/api/ws/game", makeWsHandler(state, HandleGameplayWs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(NotFoundErrorJSON)
	})

	fmt.Fprintf(util.LogWriter, "%s", sb.String())
	return mux
}
