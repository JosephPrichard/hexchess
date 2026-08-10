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
	*file.OrphanService
	*file.ProfileService
	*session.SessionService
	*user.ActiveUserService
	*gameplay.ChatService
	*tournament.TournamentService
	*tournament.TournamentBroadcaster
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

	userSvc := user.NewUserService(setup.Database)
	replaySvc := replay.NewReplayService(setup.Database)
	replaySearchSvc := replay.NewSearchService(setup.Database)
	challengeSvc := challenge.NewChallengeService(setup.Database, setup.Entropy)

	leaderboardSvc := leaderboard.NewLeaderboardService(setup.Redis, setup.Database.Querier())

	chessMetaSvc := gamestate.NewChessMetaService(setup.Database, setup.Entropy, setup.Broadcaster)
	chessRepoSvc := gamestate.NewChessRepoService(setup.Redis)
	gameplaySvc := gameplay.NewGameplayService(setup.Redis, chessRepoSvc)

	orphanSvc := file.NewOrphanService(setup.AWS, setup.Database.Querier())
	profileSvc := file.NewProfileService(setup.AWS, setup.Dispatcher, setup.Entropy)

	sessionSvc := session.NewSessionService(setup.Redis)

	tournamentSvc := tournament.NewTournamentService(setup.Database, setup.Redis, riverProducer)
	tournamentBroadcaster := tournament.NewTournamentBroadcaster(leaderboardSvc, setup.Broadcaster)

	activeUserSvc := user.NewActiveUserService(setup.Redis, setup.Broadcaster)
	chatSvc := gameplay.NewChatService(setup.Redis, setup.Database.Querier(), setup.Entropy)

	participantSvc := persona.NewPersonaService(userSvc, leaderboardSvc, replaySearchSvc)

	authTokenSvc := session.NewAuthTokenService(setup.SDKs)

	return Services{
		UserService:           userSvc,
		ReplayService:         replaySvc,
		ReplaySearchService:   replaySearchSvc,
		ChallengeService:      challengeSvc,
		LeaderboardService:    leaderboardSvc,
		ChessMetaService:      chessMetaSvc,
		ChessRepoService:      chessRepoSvc,
		GamePlayService:       gameplaySvc,
		OrphanService:         orphanSvc,
		ProfileService:        profileSvc,
		SessionService:        sessionSvc,
		TournamentService:     tournamentSvc,
		TournamentBroadcaster: tournamentBroadcaster,
		ActiveUserService:     activeUserSvc,
		ChatService:           chatSvc,
		PersonaService:        participantSvc,
		AuthTokenService:      authTokenSvc,
	}
}
