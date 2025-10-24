package web

import (
	"fmt"
	"hexchess-svc/data"
	"net/http"
	"strings"
)

type ServerState struct {
	data.Stores
	ActiveCaster *data.UniCaster
	GamesCaster  *data.UniCaster
	UsersCaster  *data.MultiCasterMap
	InitialBoard []byte
	CountryList  []byte
}

func MakeServerState(stores data.Stores, initialBoard []byte, countryList []byte) ServerState {
	return ServerState{
		Stores:       stores,
		ActiveCaster: data.MakeUniCaster("active-caster"),
		GamesCaster:  data.MakeUniCaster("games-caster"),
		UsersCaster:  data.MakeMultiCasterMap("users-caster"),
		InitialBoard: initialBoard,
		CountryList:  countryList,
	}
}

func HandleRoot(state ServerState) http.Handler {
	mux := http.NewServeMux()

	var sb strings.Builder
	sb.WriteString("starting rest server...\n")

	logRoute := func(pattern string) {
		sb.WriteString("\t")
		sb.WriteString(pattern)
		sb.WriteString("\n")
	}

	for _, route := range []struct {
		pattern string
		handler RestHandler
	}{
		{"POST /api/register", HandleRegister},
		{"POST /api/login", HandleLogin},
		{"GET /api/session/temp", HandleCreateTempSession},
		{"POST /api/session/refresh", HandleRefreshSession},
		{"POST /api/session/logout", HandleLogout},
		{"POST /api/users/password", HandleUpdatePassword},
		{"POST /api/users", HandleUpdateUser},
		{"POST /api/games/create", HandleCreateGame},
		{"POST /api/challenges/update", HandleUpdateChallenge},
		{"POST /api/challenges/create", HandleCreateChallenge},
		{"GET /api/players", HandleGetPlayer},
		{"GET /api/players/self", HandleGetSelf},
		{"GET /api/players/search", HandleSearchPlayers},
		{"GET /api/leaderboard", HandleGetLeaderboard},
		{"GET /api/challenges", HandleGetChallenges},
		{"GET /api/replays", HandleGetUserReplays},
		{"GET /api/chess/rooms", HandleGetChessRoomList},
		{"GET /api/replay", HandleGetReplay},
		{"GET /api/replay/move-list", HandleGetReplayMoveList},
	} {
		mux.Handle(route.pattern, makeRestHandler(state, route.handler))
		logRoute(route.pattern)
	}

	for _, route := range []struct {
		pattern string
		handler SseHandler
	}{
		{"GET /api/events/counts", HandleCountEvents},
		{"GET /api/events/users", HandleUserEvents},
	} {
		mux.Handle(route.pattern, makeSseHandler(state, route.handler))
		logRoute(route.pattern)
	}

	for _, route := range []struct {
		pattern  string
		resource []byte
	}{
		{"GET /api/initial-board", state.InitialBoard},
		{"GET /api/countries", state.CountryList},
	} {
		mux.Handle(route.pattern, makeStaticHandler(route.resource))
		logRoute(route.pattern)
	}

	fmt.Println(sb.String())
	return mux
}
