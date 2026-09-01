package network

import (
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/challenge"
	"hexchess-svc/service/file"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/service/persona"
	"hexchess-svc/service/replay"
	"hexchess-svc/service/session"
	"hexchess-svc/service/tournament"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
)

type Services struct {
	*user.UserService
	*challenge.ChallengeService
	*replay.ReplayService
	*replay.ReplaySearchService
	*leaderboard.LeaderboardService
	*gamestate.ChessMetaService
	*gamestate.ChessRepoService
	*gameplay.GamePlayService
	*gameplay.GameCreateService
	*file.OrphanService
	*file.ProfileService
	*session.SessionService
	*user.ActiveUserService
	*gameplay.ChatService
	*tournament.UpdateTournamentService
	*tournament.RetrieveTournamentService
	*tournament.TournamentNotificationService
	*persona.PersonaService
	*session.AuthTokenService
}

type SetupAPIServices struct {
	Database    database.Database
	RiverClient database.RiverClientAPI
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
	AWS         cloud.AWSClient
	SDKs        cloud.SDKs
	Entropy     entropy.Generator
	Dispatcher  async.Dispatcher
}

func NewAPIServices(setup SetupAPIServices) Services {
	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	riverProducer := producers.NewRiverProducer(setup.RiverClient)

	userService := user.NewUserService(setup.Database)
	replayService := replay.NewReplayService(setup.Database)
	replaySearchService := replay.NewSearchService(setup.Database)
	challengeService := challenge.NewChallengeService(setup.Database, setup.Entropy)

	leaderboardService := leaderboard.NewLeaderboardService(setup.Redis, setup.Database.Querier())

	chessMetaService := gamestate.NewChessMetaService(setup.Database, setup.Entropy, setup.Broadcaster)
	chessRepoService := gamestate.NewChessRepoService(setup.Redis)
	gameplayService := gameplay.NewGameplayService(setup.Redis, chessRepoService)
	gameCreateService := gameplay.NewGameCreateService(setup.Redis, chessRepoService)

	orphanService := file.NewOrphanService(setup.AWS, setup.Database.Querier())
	profileService := file.NewProfileService(setup.AWS, setup.Dispatcher, setup.Entropy)

	sessionService := session.NewSessionService(setup.Redis)

	updateTournamentService := tournament.NewUpdateTournamentService(setup.Database, setup.Redis, riverProducer)
	retrieveTournamentService := tournament.NewRetrieveTournamentService(setup.Database.Querier(), setup.Redis)
	tournamentBroadcaster := tournament.NewTournamentBroadcaster(leaderboardService, setup.Broadcaster)

	activeUserService := user.NewActiveUserService(setup.Redis, setup.Broadcaster)
	chatService := gameplay.NewChatService(setup.Redis, setup.Database.Querier(), setup.Entropy)

	participantService := persona.NewPersonaService(userService, leaderboardService, replaySearchService)

	authTokenService := session.NewAuthTokenService(setup.SDKs)

	return Services{
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
