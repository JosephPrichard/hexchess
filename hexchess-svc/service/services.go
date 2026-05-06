package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/egress"
	"hexchess-svc/hexchess"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"io"
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
	VerifyUserTx(ctx context.Context, username string, inputPassword string) (VerifiedUser, error)
	SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error)
	ValidateGoogleIDToken(ctx context.Context, token string) (egress.GoogleIDTokenPayload, error)
	UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (model.User, error)
	GetUserByID(ctx context.Context, id int64) (model.User, error)
	GetUserStats(ctx context.Context, id int64) (model.UserStats, error)
	GetFullUser(ctx context.Context, userID int64, perPage int32) (FullUser, error)
	UpdateUserPassword(ctx context.Context, id int64, newPassword string) error
	SelectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error)

	SetLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error
	GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (map[string]LbRank, error)
	SyncLeaderboard(ctx context.Context) error
	GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error)
	GetLeaderboardUsers(ctx context.Context, mode model.GameMode, rnkUsers []RankedUser) ([]model.LbdUser, []int64, error)
	GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) ([]model.LbdUser, error)
	GetLeaderboardPage(ctx context.Context, mode model.GameMode, page, perPage int64) (Leaderboard, error)

	GetReplayByGameID(ctx context.Context, gameID string) (model.FullReplay, error)
	GetReplay(ctx context.Context, replayID int64) (model.FullReplay, error)
	GetUserReplays(ctx context.Context, replayQuery ReplayQuery, perPage int32) ([]model.FullReplay, error)
	GetMovesHistory(ctx context.Context, replayID int) ([]byte, error)
	RetrieveEloHistoryBuckets(ctx context.Context, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error)

	InsertChallenge(ctx context.Context, inst ChallengeInst) (model.Challenge, error)
	GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]model.Challenge, error)
	DeleteChallenge(ctx context.Context, key ChallengeKey) (DeleteResult, error)
	DeleteExpiredChallenges(ctx context.Context, userID int64) error
	CountUserChallenges(ctx context.Context, userID int64) (int64, error)

	MakeProfileURL(key string) string
	GetProfilePicKey(ctx context.Context, userID string) (string, error)
	UploadProfilePic(ctx context.Context, uploader model.PlayerState, file io.Reader, contentType string) (string, error)
	DeleteOldProfilePics(ctx context.Context, playerID int) error

	ClearOrphanFiles(ctx context.Context, pageLength int32)

	GetTournament(ctx context.Context, tournamentKey uuid.UUID) (model.FullTournament, error)
	GetTournaments(ctx context.Context, participantID int64, afterID int64, perPage int32) ([]model.Tournament, error)
	CreateTournamentTx(ctx context.Context, inst TournamentInst) (int64, error)
	JoinTournamentTx(ctx context.Context, inst JoinTournamentInst) (JoinTournamentResult, error)
	BeginTournamentCountdownTx(ctx context.Context, tournamentKey uuid.UUID, userID int64) (BeginTourneyCountdown, error)
	LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error)
	AdvanceTournamentTx(ctx context.Context, tournamentKey uuid.UUID) error

	GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error)
	GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error)
	GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error)
	GetChessStateCount(ctx context.Context) (int64, error)
	SetManyChessStates(ctx context.Context, chessStates []ChessState) error

	GetStateChats(ctx context.Context, gameID string, count int64) ([]*pb.ChatMessage, error)
	InsertStateChat(ctx context.Context, gameID string, chat Chat) error

	IsActiveUser(ctx context.Context, id string) bool
	GetActiveCount(ctx context.Context) (int64, error)
	RetainActiveUser(ctx context.Context, id string) error
	AddActiveUser(ctx context.Context, id string) (int64, error)
	RemoveActiveUser(ctx context.Context, id string) (int64, error)

	CreateGame(ctx context.Context, color model.GameColor, mode model.GameMode, initialBoard *hexchess.Board) (string, error)
	JoinGame(ctx context.Context, gameID string, player model.PlayerState) (*ChessState, error)
	MakeGameMove(ctx context.Context, gameID string, player model.PlayerState, move hexchess.Move) (MoveResult, error)
	AttemptGameUndo(ctx context.Context, gameID string, player model.PlayerState, kind UndoKind) (*ChessState, error)
	EndGame(ctx context.Context, gameID string, player model.PlayerState) (EndKind, error)
	IsGameAccessible(ctx context.Context, id string) bool
	InsertFinishedGame(ctx context.Context, finishedGame FinishedGame) error
	InsertGameResultTx(ctx context.Context, params GameResult) (GameResultChangeSet, error)
	UpsertReplayMoveHistories(ctx context.Context, replayID int64, data []byte) error

	BroadcastActiveCount(ctx context.Context, count int64) error
	BroadcastGameCount(ctx context.Context, count int64) error
	BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) error
	BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) error
	BroadcastChallenge(ctx context.Context, challenge model.Challenge) error
}

type HexchessServices struct {
	db      db.DB
	querier sqlc.Querier
	redis   db.Redis
	aws     egress.AWS
	remote  egress.RemoteAPIs
	entropy EntropySource
}

var _ = (HexchessAPI)(&HexchessServices{})

func (svc *HexchessServices) Close() {
	if svc.db != nil {
		svc.db.Close()
	}
	svc.redis.Close()
}

type Setup struct {
	DB      db.DB
	Querier sqlc.Querier
	Redis   db.Redis
	AWS     egress.AWS
	Remote  egress.RemoteAPIs
	Entropy EntropySource
}

func MakeHexchessServices(setup Setup) *HexchessServices {
	if setup.DB != nil {
		setup.Querier = setup.DB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = &RealEntropySource{}
	}
	return &HexchessServices{
		db:      setup.DB,
		querier: setup.Querier,
		redis:   setup.Redis,
		aws:     setup.AWS,
		remote:  setup.Remote,
		entropy: setup.Entropy,
	}
}
