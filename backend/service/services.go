package svc

import (
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
)

type HexchessServices struct {
	*UserService
	*ChallengeService
	*ReplayService
	*LeaderboardService
	*ChessMetaService
	*ChessRepoService
	*GameOverService
	*GamePlayService
	*OrphanService
	*ProfileService
	*SessionService
	*ActiveUserService
	*ChatService
	*TournamentService
	*FullUserService
	*AuthTokenService
}

type SetupService struct {
	// postgres infra
	Database    db.Database
	RiverClient producers.RiverClientAPI
	// redis infra
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
	// remote and local cloud sdks / clients
	AWS  cloud.AWSClient
	SDKs cloud.SDKs
	// non-deterministic behaviors for easy mocking
	Entropy    entropy.Generator
	Dispatcher async.Dispatcher
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	database := setup.Database
	transactor := &setup.Database

	mutator := database.Querier()
	querier := database.ReadQuerier()
	riverProducer := producers.NewRiverProducer(setup.RiverClient)
	streamProducer := producers.NewPublisher(setup.Redis)

	userService := NewUserService(transactor, mutator, querier)
	replayService := NewReplayService(userService, mutator, querier)
	challengeService := NewChallengeService(mutator, querier, setup.Entropy)

	leaderboardService := NewLeaderboardService(setup.Redis, querier)

	chessMetaService := NewChessMetaService(mutator, querier, setup.Entropy, setup.Broadcaster)
	chessRepoService := NewChessRepoService(setup.Redis)
	gameOverService := NewGameOverService(replayService, leaderboardService, transactor, querier, riverProducer, setup.Broadcaster)
	gamePlayService := NewGamePlayService(chessRepoService, streamProducer)

	orphanService := NewOrphanService(setup.AWS, querier)
	profileService := NewProfileService(setup.AWS, setup.Dispatcher, setup.Entropy)

	sessionService := NewSessionService(setup.Redis)
	tournamentService := NewTournamentService(leaderboardService, userService, gamePlayService, transactor, mutator, querier, riverProducer, setup.Broadcaster)

	activeUserService := NewActiveUserService(setup.Redis, setup.Broadcaster)
	chatService := NewChatService(setup.Redis, querier, setup.Entropy)

	fullUserService := NewFullUserService(userService, leaderboardService, replayService)

	authTokenService := NewAuthTokenService(setup.SDKs)

	return &HexchessServices{
		UserService:        userService,
		ReplayService:      replayService,
		ChallengeService:   challengeService,
		LeaderboardService: leaderboardService,
		ChessMetaService:   chessMetaService,
		ChessRepoService:   chessRepoService,
		GameOverService:    gameOverService,
		GamePlayService:    gamePlayService,
		OrphanService:      orphanService,
		ProfileService:     profileService,
		SessionService:     sessionService,
		TournamentService:  tournamentService,
		ActiveUserService:  activeUserService,
		ChatService:        chatService,
		FullUserService:    fullUserService,
		AuthTokenService:   authTokenService,
	}
}
