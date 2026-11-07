package network

import (
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/challenges"
	"hexchess-svc/chess"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/game"
	"hexchess-svc/leaderboard"
	"hexchess-svc/persona"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/queue/producers"
	"hexchess-svc/replay"
	"hexchess-svc/session"
	"hexchess-svc/tournament"
	"hexchess-svc/user"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"log/slog"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type HttpServer struct {
	service       HttpServices
	broadcasters  *subscribers.LocalBroadcasters
	broadcaster   publishers.Broadcaster
	dispatcher    async.Dispatcher
	entropy       entropy.Generator
	authenticator HttpAuthenticator
	staticData    StaticData
}

type HttpServices struct {
	*user.UserService
	*user.UserSearchService
	*challenges.ChallengeService
	*replay.ReplayService
	*replay.ReplaySearchService
	*leaderboard.LeaderboardService
	*game.ChessMetadataService
	*game.ChessRepoService
	*game.GamePlayService
	*game.GameCreateService
	*user.OrphanProfileService
	*user.ProfileService
	*session.SessionService
	*user.ActiveUserService
	*game.ChatService
	*tournament.TournamentUpdater
	*tournament.TournamentRetriever
	*tournament.TournamentNotifier
	*persona.PersonaService
	*session.AuthTokenService
}

type HttpServerConfig struct {
	Database        database.PostgreSQL
	RiverClient     database.RiverClientAPI
	RealRiverClient *river.Client[pgx.Tx]
	Redis           cache.Redis
	Broadcaster     publishers.Broadcaster
	AWS             cloud.AWSClient
	SDKs            cloud.SDKs
	EsClient        *elasticsearch.TypedClient
	Entropy         entropy.Generator
	Dispatcher      async.Dispatcher
	Broadcasters    *subscribers.LocalBroadcasters
	AllowedOrigins  string
}

func NewServeMux(config HttpServerConfig, opts ...func(*chi.Mux)) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(RouteMiddleware(config.AllowedOrigins))

	if config.Entropy == nil {
		config.Entropy = entropy.RealSource{}
	}
	if config.Dispatcher == nil {
		config.Dispatcher = async.AsyncDispatcher{}
	}

	server := NewHttpServer(config)

	r.Post("/api/register", Rest(server.HandleRegister))
	r.Post("/api/login", Rest(server.HandleLogin))
	r.Post("/api/login/google", Rest(server.HandleGoogleLogin))
	r.Post("/api/logout", Rest(server.HandleLogout))
	r.Post("/api/session/temp", Rest(server.HandleCreateTempSession))
	r.Post("/api/session/refresh", Rest(server.HandleRefreshSession))
	r.Post("/api/users/password", Rest(server.HandleUpdatePassword))
	r.Post("/api/users", Rest(server.HandleUpdateUser))
	r.Post("/api/games/create", Rest(server.HandleCreateGame))
	r.Post("/api/challenges/update", Rest(server.HandleUpdateChallenge))
	r.Post("/api/challenges/create", Rest(server.HandleCreateChallenge))
	r.Post("/api/users/profile-pics", Rest(server.HandleUploadProfilePic))

	r.Get("/api/players", Rest(server.HandleGetPersona))
	r.Get("/api/players/self", Rest(server.HandleGetSelf))
	r.Get("/api/players/search", Rest(server.HandleSearchPlayers))
	r.Get("/api/players/activity", Rest(server.HandleUserActivityCheck))
	r.Get("/api/leaderboard", Rest(server.HandleGetLeaderboard))
	r.Get("/api/challenges", Rest(server.HandleGetChallenges))
	r.Get("/api/challenges/count", Rest(server.HandleCountUserChallenges))
	r.Get("/api/replays", Rest(server.HandleSearchReplays))
	r.Get("/api/replay", Rest(server.HandleGetReplay))
	r.Get("/api/replay/elo-histories", Rest(server.HandleGetEloHistories))
	r.Get("/api/replay/move-list", Rest(server.HandleGetMoveReplay))
	r.Get("/api/game/rooms", Rest(server.HandleGetGameMetadata))
	r.Get("/api/game/rooms/chats", Rest(server.HandleGetGameChats))
	r.Get("/api/game/rooms/exists", Rest(server.HandleGameExistence))
	r.Get("/api/users/profile-pics", Rest(server.HandleGetProfilePic))
	r.Get("/api/tournament", Rest(server.HandleGetTournament))
	r.Get("/api/tournaments", Rest(server.HandleGetTournaments))

	r.Get("/api/events/count", SSE(server.HandleCountEvents))
	r.Get("/api/events/user", SSE(server.HandleUserEvents))
	r.Get("/api/events/active", SSE(server.HandleActiveConn))
	r.Get("/api/events/tournament", SSE(server.HandleTournamentEvents))

	r.Get("/api/initial-board", Json(chess.InitialBoard()))
	r.Get("/api/countries", Json(server.staticData.countryList))

	r.Get("/api/ws/game", server.HandleGameWebSocket)
	r.Get("/api/info", info)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "route not found", "method", r.Method, "url", r.URL.String())
		writeJSON(w, http.StatusNotFound, ServiceResp{Status: http.StatusNotFound, Message: "ROUTE_NOT_FOUND"})
	})

	SetHealthCheck(r, HealthConfig{
		PostgresCheck:     config.Database.HealthcheckFunc(),
		PostgresReadCheck: config.Database.ReadHealthcheckFunc(),
		RedisPrimaryCheck: config.Redis.PrimaryHealthCheck,
		RedisPubSubCheck:  config.Redis.PubsubHealthCheck,
	})
	SetAdminEndpoints(r, config.RealRiverClient)

	for _, opt := range opts {
		opt(r)
	}

	var handlers []string
	_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		handlers = append(handlers, fmt.Sprintf("%s %s", method, route))
		return nil
	})
	slog.Info("initialized serve mux", "handlers", handlers)

	return r
}

func NewHttpServer(config HttpServerConfig) HttpServer {
	return HttpServer{
		service:       NewHttpServices(config),
		broadcaster:   config.Broadcaster,
		broadcasters:  config.Broadcasters,
		dispatcher:    config.Dispatcher,
		entropy:       config.Entropy,
		authenticator: HttpAuthenticator{service: session.NewSessionService(config.Redis)},
		staticData:    NewStaticData(),
	}
}

func NewHttpServices(config HttpServerConfig) HttpServices {
	// set default values
	if config.Entropy == nil {
		config.Entropy = entropy.RealSource{}
	}
	if config.Dispatcher == nil {
		config.Dispatcher = async.AsyncDispatcher{}
	}

	// initialize services
	riverProducer := producers.NewRiverProducer(config.RiverClient)
	userService := user.NewUserService(config.Database, riverProducer)
	userSearchService := user.NewUserSearchService(config.Database.Querier, config.EsClient)
	replayService := replay.NewReplayService(config.Database)
	replaySearchService := replay.NewReplaySearchService(config.Database)
	challengeService := challenges.NewChallengeService(config.Database, config.Entropy)
	leaderboardService := leaderboard.NewLeaderboardService(config.Redis, config.Database.Querier)
	chessMetaService := game.NewChessMetadataService(config.Database, config.Entropy, config.Broadcaster)
	chessRepoService := game.NewChessRepoService(config.Redis)
	gameplayService := game.NewGameplayService(config.Redis, chessRepoService)
	gameCreateService := game.NewGameCreateService(config.Redis, chessRepoService)
	orphanService := user.NewOrphanService(config.AWS, config.Database.Querier)
	profileService := user.NewProfileService(config.AWS, config.Dispatcher, config.Entropy)
	sessionService := session.NewSessionService(config.Redis)
	tournamentUpdater := tournament.NewTournamentUpdater(config.Database, config.Redis, riverProducer)
	tournamentRetriever := tournament.NewRetrieveTournamentService(config.Database.Querier, config.Redis)
	tournamentNotifier := tournament.NewTournamentNotifier(leaderboardService, config.Broadcaster)
	activeUserService := user.NewActiveUserService(config.Redis, config.Broadcaster)
	chatService := game.NewChatService(config.Redis, config.Database.Querier, config.EsClient, config.Entropy, config.Dispatcher)
	personaService := persona.NewPersonaService(config.Database, config.Redis)
	authTokenService := session.NewAuthTokenService(config.SDKs)

	return HttpServices{
		UserService:          userService,
		UserSearchService:    userSearchService,
		ReplayService:        replayService,
		ReplaySearchService:  replaySearchService,
		ChallengeService:     challengeService,
		LeaderboardService:   leaderboardService,
		ChessMetadataService: chessMetaService,
		ChessRepoService:     chessRepoService,
		GamePlayService:      gameplayService,
		GameCreateService:    gameCreateService,
		OrphanProfileService: orphanService,
		ProfileService:       profileService,
		SessionService:       sessionService,
		TournamentUpdater:    tournamentUpdater,
		TournamentRetriever:  tournamentRetriever,
		TournamentNotifier:   tournamentNotifier,
		ActiveUserService:    activeUserService,
		ChatService:          chatService,
		PersonaService:       personaService,
		AuthTokenService:     authTokenService,
	}
}
