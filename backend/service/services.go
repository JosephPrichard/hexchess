package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/egress"
	"hexchess-svc/lib/enum"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"io"

	"hexchess-svc/model"
	"time"

	"github.com/google/uuid"
)

//go:generate mockgen -source=services.go -destination=./services_mock.go -package=svc

type HexchessAPI interface {
	SetSessions(ctx context.Context, insts ...SessionInst) error
	GetSession(ctx context.Context, sessionID string) (model.PlayerState, error)
	DeleteSession(ctx context.Context, sessionID string) error
	UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error

	InsertUser(ctx context.Context, inst UserInst) (model.User, error)
	BatchInsertUsers(ctx context.Context, insts []UserInst) ([]model.User, error)
	VerifyUser(ctx context.Context, username string, inputPassword string) (VerifiedUser, error)
	SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error)
	ValidateGoogleIDToken(ctx context.Context, token string) (egress.GoogleIDTokenPayload, error)
	UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (model.User, error)
	GetUserByID(ctx context.Context, id int64) (model.User, error)
	GetUserStats(ctx context.Context, id int64) (model.UserStats, error)
	GetFullUser(ctx context.Context, userID int64, perPage int32) (FullUser, error)
	UpdateUserPassword(ctx context.Context, id int64, newPassword string) error

	SetLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error
	GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (map[string]LbRank, error)
	SyncLeaderboard(ctx context.Context) error
	GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error)
	GetFullLeaderboardUsers(ctx context.Context, mode model.GameMode, rnkUsers []RankedUser) ([]model.LbdUser, []int64, error)
	GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) ([]model.LbdUser, error)
	GetLeaderboardPage(ctx context.Context, mode model.GameMode, page, perPage int64) (Leaderboard, error)

	GetReplayByGameID(ctx context.Context, gameID string) (model.FullReplay, error)
	GetReplay(ctx context.Context, replayID int64) (model.FullReplay, error)
	SearchReplaysByQuery(ctx context.Context, query ReplaysQuery) ([]model.FullReplay, error)
	GetMovesHistory(ctx context.Context, replayID int) ([]byte, error)
	RetrieveEloHistoryBuckets(ctx context.Context, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error)

	InsertChallenge(ctx context.Context, inst ChallengeInst) (model.Challenge, error)
	BatchInsertChallenges(ctx context.Context, insts []ChallengeInst) error
	GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]model.Challenge, error)
	DeleteChallenge(ctx context.Context, key ChallengeKey) (DeleteResult, error)
	DeleteExpiredChallenges(ctx context.Context, userID int64) error
	CountUserChallenges(ctx context.Context, userID int64) (int64, error)

	MakeProfileURL(key string) string
	GetProfilePicKey(ctx context.Context, userID string) (string, error)
	UploadProfilePic(ctx context.Context, uploader model.PlayerState, file io.ReadCloser, contentType string, contentChecksum string) (UploadProfileResult, error)

	ClearOrphanFiles(ctx context.Context, pageLength int32)

	GetTournament(ctx context.Context, tournamentKey uuid.UUID) (model.FullTournament, error)
	GetTournaments(ctx context.Context, participantID enum.Optional[int64], afterID enum.Optional[int64], perPage int32) ([]model.Tournament, error)
	CreateTournament(ctx context.Context, inst TournamentInst) (int64, error)
	JoinTournament(ctx context.Context, inst JoinTournamentInst) (JoinTournamentEvent, error)
	BeginTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error)
	LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error)
	AdvanceTournament(ctx context.Context, tournamentKey uuid.UUID, eventID uuid.UUID) ([]string, error)

	BroadcastTournamentParticipant(ctx context.Context, playerID int64, tournamentJoin JoinTournamentEvent)

	UpdateGameMetadata(ctx context.Context, updt model.GameMetadataUpdt) error
	GetGameMetadata(ctx context.Context, player enum.Optional[model.PlayerState], afterOrdering enum.Optional[int64], count int32) (ChessMetasResp, error)
	GetGameMetadataCount(ctx context.Context) (int64, error)

	GetChats(ctx context.Context, gameID string, count int64) ([]model.Chat, error)
	InsertChat(ctx context.Context, gameID string, chat model.Chat) error

	IsActiveUser(ctx context.Context, id string) bool
	GetActiveCount(ctx context.Context) (int64, error)
	RetainActiveUser(ctx context.Context, id string) error
	AddActiveUser(ctx context.Context, id string) (int64, error)
	RemoveActiveUser(ctx context.Context, id string) (int64, error)

	CreateGame(ctx context.Context, color model.GameColor, mode model.GameMode, initialBoard *chess.Board) (string, error)
	JoinGame(ctx context.Context, gameID string, player model.PlayerState) (*model.ChessState, error)
	MakeGameMove(ctx context.Context, gameID string, player model.PlayerState, move chess.Move) (MoveResult, error)
	AttemptGameUndo(ctx context.Context, gameID string, player model.PlayerState, kind UndoKind) (*model.ChessState, error)
	EndGame(ctx context.Context, gameID string, player model.PlayerState) (model.EndKind, error)
	IsGameAccessible(ctx context.Context, id string) bool
	InsertFinishedGame(ctx context.Context, finishedGame model.FinishedGame) error
	InsertGameResult(ctx context.Context, params GameResult) (GameResultChangeSet, error)
	UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error
}

type HexchessServices struct {
	transactor     db.Transactor
	querier        sqlc.Querier
	redis          db.Redis
	aws            egress.AWS
	remote         egress.RemoteAPIs
	broadcaster    pubsub.BroadcasterAPI
	redisPublisher producers.RedisPublisher
	entropy        EntropyAPI
}

//var _ = (HexchessAPI)(&HexchessServices{})

func (services *HexchessServices) Close() {
	if services.transactor != nil {
		services.transactor.Close()
	}
	services.redis.Close()
}

type SetupService struct {
	DB          db.DB
	Redis       db.Redis
	AWS         egress.AWS
	Remote      egress.RemoteAPIs
	Entropy     EntropyAPI
	Broadcaster pubsub.BroadcasterAPI
}

func MakeHexchessServices(setup SetupService) *HexchessServices {
	var querier sqlc.Querier
	if setup.DB != nil {
		querier = setup.DB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = &RealEntropySource{}
	}
	return &HexchessServices{
		transactor:     setup.DB,
		querier:        querier,
		redis:          setup.Redis,
		aws:            setup.AWS,
		remote:         setup.Remote,
		entropy:        setup.Entropy,
		redisPublisher: producers.MakePublisher(setup.Redis),
		broadcaster:    setup.Broadcaster,
	}
}
