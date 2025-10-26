package web

import (
	"fmt"
	"hexchess-svc/data"
	"net/http"
	"strings"
)

type ServerState struct {
	data.Stores
	ActiveCntCaster *data.UniCaster
	GamesCntCaster  *data.UniCaster
	GamesCaster     *data.MultiCasterMap
	UsersCaster     *data.MultiCasterMap
	InitialBoard    []byte
	CountryList     []byte
}

func MakeServerState(stores data.Stores, initialBoard []byte, countryList []byte) ServerState {
	return ServerState{
		Stores:          stores,
		ActiveCntCaster: data.MakeUniCaster("active-cnt-caster"),
		GamesCntCaster:  data.MakeUniCaster("games-cnt-caster"),
		GamesCaster:     data.MakeMultiCasterMap("games-caster"),
		UsersCaster:     data.MakeMultiCasterMap("users-caster"),
		InitialBoard:    initialBoard,
		CountryList:     countryList,
	}
}

func HandleRoot(state ServerState) http.Handler {
	mux := http.NewServeMux()
	var sb strings.Builder

	sb.WriteString("starting rest server...\n")

	handle := func(pattern string, handler http.Handler) {
		mux.Handle(pattern, handler)
		sb.WriteString("\t")
		sb.WriteString(pattern)
		sb.WriteString("\n")
	}

	handle("POST /api/register", makeRestHandler(state, HandleRegister))
	handle("POST /api/login", makeRestHandler(state, HandleLogin))
	handle("GET /api/session/temp", makeRestHandler(state, HandleCreateTempSession))
	handle("POST /api/session/refresh", makeRestHandler(state, HandleRefreshSession))
	handle("POST /api/session/logout", makeRestHandler(state, HandleLogout))
	handle("POST /api/users/password", makeRestHandler(state, HandleUpdatePassword))
	handle("POST /api/users", makeRestHandler(state, HandleUpdateUser))
	handle("POST /api/games/create", makeRestHandler(state, HandleCreateGame))
	handle("POST /api/challenges/update", makeRestHandler(state, HandleUpdateChallenge))
	handle("POST /api/challenges/create", makeRestHandler(state, HandleCreateChallenge))
	handle("GET /api/players", makeRestHandler(state, HandleGetPlayer))
	handle("GET /api/players/self", makeRestHandler(state, HandleGetSelf))
	handle("GET /api/players/search", makeRestHandler(state, HandleSearchPlayers))
	handle("GET /api/leaderboard", makeRestHandler(state, HandleGetLeaderboard))
	handle("GET /api/challenges", makeRestHandler(state, HandleGetChallenges))
	handle("GET /api/replays", makeRestHandler(state, HandleGetUserReplays))
	handle("GET /api/chess/rooms", makeRestHandler(state, HandleGetChessRoomList))
	handle("GET /api/replay", makeRestHandler(state, HandleGetReplay))
	handle("GET /api/replay/move-list", makeRestHandler(state, HandleGetReplayMoveList))

	handle("/api/ws/game", makeWsHandler(state, HandleGameplayWs))

	handle("GET /api/events/counts", makeSseHandler(state, HandleCountEvents))
	handle("GET /api/events/users", makeSseHandler(state, HandleUserEvents))

	handle("GET /api/initial-board", makeStaticHandler(state.InitialBoard))
	handle("GET /api/countries", makeStaticHandler(state.CountryList))

	fmt.Println(sb.String())
	return mux
}
