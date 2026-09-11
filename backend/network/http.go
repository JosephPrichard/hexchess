package network

import (
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type HttpServer struct {
	services      HttpServices
	broadcasters  *pubsub.LocalBroadcasters
	broadcaster   pubsub.Broadcaster
	dispatcher    async.Dispatcher
	entropy       entropy.Generator
	authenticator HttpAuthenticator
	staticData    StaticData
}

type HttpServices struct {
	*service.UserService
	*service.ChallengeService
	*service.ReplayService
	*service.ReplaySearchService
	*service.LeaderboardService
	*service.ChessMetaService
	*service.ChessRepoService
	*service.GamePlayService
	*service.GameCreateService
	*service.OrphanService
	*service.ProfileService
	*service.SessionService
	*service.ActiveUserService
	*service.ChatService
	*service.UpdateTournamentService
	*service.RetrieveTournamentService
	*service.TournamentNotificationService
	*service.PersonaService
	*service.AuthTokenService
}

type HttpServerConfig struct {
	Database       database.Database
	RiverClient    database.RiverClientAPI
	Redis          cache.Redis
	Broadcaster    pubsub.Broadcaster
	AWS            cloud.AWSClient
	SDKs           cloud.SDKs
	Entropy        entropy.Generator
	Dispatcher     async.Dispatcher
	Broadcasters   *pubsub.LocalBroadcasters
	AllowedOrigins string
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

	server := NewHttpServer(HttpServerConfig{
		Database:       config.Database,
		RiverClient:    config.RiverClient,
		Redis:          config.Redis,
		Broadcaster:    config.Broadcaster,
		AWS:            config.AWS,
		SDKs:           config.SDKs,
		Entropy:        config.Entropy,
		Dispatcher:     config.Dispatcher,
		Broadcasters:   config.Broadcasters,
		AllowedOrigins: config.AllowedOrigins,
	})

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
		services:     NewHttpServices(config),
		broadcaster:  config.Broadcaster,
		broadcasters: config.Broadcasters,
		dispatcher:   config.Dispatcher,
		entropy:      config.Entropy,
		authenticator: HttpAuthenticator{
			services: service.NewSessionService(config.Redis),
		},
		staticData: NewStaticData(),
	}
}

func NewHttpServices(setup HttpServerConfig) HttpServices {
	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	riverProducer := producers.NewRiverProducer(setup.RiverClient)

	userService := service.NewUserService(setup.Database)
	replayService := service.NewReplayService(setup.Database)
	replaySearchService := service.NewSearchService(setup.Database)
	challengeService := service.NewChallengeService(setup.Database, setup.Entropy)

	leaderboardService := service.NewLeaderboardService(setup.Redis, setup.Database.Querier())

	chessMetaService := service.NewChessMetaService(setup.Database, setup.Entropy, setup.Broadcaster)
	chessRepoService := service.NewChessRepoService(setup.Redis)
	gameplayService := service.NewGameplayService(setup.Redis, chessRepoService)
	gameCreateService := service.NewGameCreateService(setup.Redis, chessRepoService)

	orphanService := service.NewOrphanService(setup.AWS, setup.Database.Querier())
	profileService := service.NewProfileService(setup.AWS, setup.Dispatcher, setup.Entropy)

	sessionService := service.NewSessionService(setup.Redis)

	updateTournamentService := service.NewUpdateTournamentService(setup.Database, setup.Redis, riverProducer)
	retrieveTournamentService := service.NewRetrieveTournamentService(setup.Database.Querier(), setup.Redis)
	tournamentBroadcaster := service.NewTournamentBroadcaster(leaderboardService, setup.Broadcaster)

	activeUserService := service.NewActiveUserService(setup.Redis, setup.Broadcaster)
	chatService := service.NewChatService(setup.Redis, setup.Database.Querier(), setup.Entropy)

	participantService := service.NewPersonaService(userService, leaderboardService, replaySearchService)

	authTokenService := service.NewAuthTokenService(setup.SDKs)

	return HttpServices{
		UserService:                   userService,
		ReplayService:                 replayService,
		ReplaySearchService:           replaySearchService,
		ChallengeService:              challengeService,
		LeaderboardService:            leaderboardService,
		ChessMetaService:              chessMetaService,
		ChessRepoService:              chessRepoService,
		GamePlayService:               gameplayService,
		GameCreateService:             gameCreateService,
		OrphanService:                 orphanService,
		ProfileService:                profileService,
		SessionService:                sessionService,
		UpdateTournamentService:       updateTournamentService,
		RetrieveTournamentService:     retrieveTournamentService,
		TournamentNotificationService: tournamentBroadcaster,
		ActiveUserService:             activeUserService,
		ChatService:                   chatService,
		PersonaService:                participantService,
		AuthTokenService:              authTokenService,
	}
}
