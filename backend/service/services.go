package svc

import (
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/challenge"
	"hexchess-svc/service/chess"
	"hexchess-svc/service/file"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/service/participant"
	"hexchess-svc/service/replay"
	"hexchess-svc/service/session"
	"hexchess-svc/service/tournament"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
)

type HexchessServices struct {
	*user.UserService
	*challenge.ChallengeService
	*replay.ReplayService
	*leaderboard.LeaderboardService
	*chess.ChessMetaService
	*chess.ChessRepoService
	*gameplay.GameOverService
	*gameplay.GamePlayService
	*file.OrphanService
	*file.ProfileService
	*session.SessionService
	*user.ActiveUserService
	*gameplay.ChatService
	*tournament.TournamentService
	*tournament.TournamentOrchestratorService
	*participant.ParticipantService
	*session.AuthTokenService
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

type Database struct {
	transactor db.Transactor
	mutator    mutator.Querier
	querier    query.Querier
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	d := setup.Database
	database := db.Operator{Transactor: &setup.Database, Mutator: d.Mutator(), Querier: d.Querier()}

	riverProducer := producers.NewRiverProducer(setup.RiverClient)
	streamProducer := producers.NewStreamProducer(setup.Redis)

	userSvc := user.NewUserService(database)
	replaySvc := replay.NewReplayService(userSvc, database)
	challengeSvc := challenge.NewChallengeService(database, setup.Entropy)

	leaderboardSvc := leaderboard.NewLeaderboardService(setup.Redis, database.Querier)

	chessMetaSvc := chess.NewChessMetaService(database, setup.Entropy, setup.Broadcaster)
	chessRepoSvc := chess.NewChessRepoService(setup.Redis)
	gameoverSvc := gameplay.NewGameoverService(replaySvc, leaderboardSvc, database, riverProducer, setup.Broadcaster)
	gameplaySvc := gameplay.NewGameplayService(chessRepoSvc, setup.Redis, streamProducer)

	orphanSvc := file.NewOrphanService(setup.AWS, database.Querier)
	profileSvc := file.NewProfileService(setup.AWS, setup.Dispatcher, setup.Entropy)

	sessionSvc := session.NewSessionService(setup.Redis)

	tournamentSvc := tournament.NewTournamentService(leaderboardSvc, database, riverProducer)
	tournamentOrchestratorSvc := tournament.NewOrchestratorService(tournamentSvc, leaderboardSvc, userSvc, gameplaySvc, setup.Broadcaster)

	activeUserSvc := user.NewActiveUserService(setup.Redis, setup.Broadcaster)
	chatSvc := gameplay.NewChatService(setup.Redis, database.Querier, setup.Entropy)

	participantSvc := participant.NewParticipantService(userSvc, leaderboardSvc, replaySvc)

	authTokenSvc := session.NewAuthTokenService(setup.SDKs)

	return &HexchessServices{
		UserService:                   userSvc,
		ReplayService:                 replaySvc,
		ChallengeService:              challengeSvc,
		LeaderboardService:            leaderboardSvc,
		ChessMetaService:              chessMetaSvc,
		ChessRepoService:              chessRepoSvc,
		GameOverService:               gameoverSvc,
		GamePlayService:               gameplaySvc,
		OrphanService:                 orphanSvc,
		ProfileService:                profileSvc,
		SessionService:                sessionSvc,
		TournamentService:             tournamentSvc,
		TournamentOrchestratorService: tournamentOrchestratorSvc,
		ActiveUserService:             activeUserSvc,
		ChatService:                   chatSvc,
		ParticipantService:            participantSvc,
		AuthTokenService:              authTokenSvc,
	}
}
