package itest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"hexchess-svc/utils/logutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// TimeNow is a stable and consistent constant we use mock ext the 'now' value in our testing data
var TimeNow = time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)

var UsersInsts = []struct {
	Username string
	Password string
	Country  string
	JoinedOn time.Time
}{
	// used for user/challenge/replay/tournament tests
	{Username: "user1", Password: "password1", Country: "us", JoinedOn: TimeNow},
	{Username: "user2", Password: "password2", Country: "us", JoinedOn: TimeNow},
	{Username: "user3", Password: "password3", Country: "us", JoinedOn: TimeNow},
	{Username: "user4", Password: "password4", Country: "us", JoinedOn: TimeNow},
	{Username: "user5", Password: "password5", Country: "us", JoinedOn: TimeNow},
	// used for elo histories tests.
	{Username: "user6", Password: "password6", Country: "us", JoinedOn: TimeNow},
	{Username: "user7", Password: "password7", Country: "us", JoinedOn: TimeNow},
	// used for search leaderboard tests.
	{Username: "john", Password: "password8", Country: "us", JoinedOn: TimeNow},
	{Username: "johnny", Password: "password9", Country: "us", JoinedOn: TimeNow},
	// used for tournament replay tests
	{Username: "user10", Password: "password10", Country: "us", JoinedOn: TimeNow},
	{Username: "user11", Password: "password11", Country: "us", JoinedOn: TimeNow},
	{Username: "user12", Password: "password12", Country: "us", JoinedOn: TimeNow},
	{Username: "user13", Password: "password13", Country: "us", JoinedOn: TimeNow},
}

var UserModeElos = []struct {
	UserID int64
	Mode   string
	Elo    int64
	Wins   int32
	Losses int32
}{
	{UserID: 3, Mode: "CORRESPONDENCE_1", Elo: 900},
	{UserID: 4, Mode: "CORRESPONDENCE_1", Elo: 2000},
	{UserID: 5, Mode: "CORRESPONDENCE_1", Elo: 1500},
	{UserID: 6, Mode: "CORRESPONDENCE_1", Elo: 1600},
	{UserID: 8, Mode: "CORRESPONDENCE_1", Elo: 1000, Wins: 2, Losses: 2},
	{UserID: 9, Mode: "CORRESPONDENCE_1", Elo: 1500, Wins: 5, Losses: 2},

	{UserID: 1, Mode: "CORRESPONDENCE_7", Elo: 1000, Wins: 2, Losses: 2},
	{UserID: 3, Mode: "CORRESPONDENCE_7", Elo: 900},
	{UserID: 8, Mode: "CORRESPONDENCE_7", Elo: 2000, Wins: 10, Losses: 2},

	{UserID: 1, Mode: "TIMED_3+2", Elo: 1020, Wins: 5, Losses: 4},

	{UserID: 1, Mode: "TIMED_15+10", Elo: 1030, Wins: 4, Losses: 3},

	{UserID: 1, Mode: "TIMED_1+0", Elo: 1050, Wins: 6, Losses: 5},
	{UserID: 2, Mode: "TIMED_1+0", Elo: 1000},
}

var FirstReplayGameID = ReplayInsts[0].GameID

const (
	FirstReplayID = 1
	GuestReplayID = 4
)

var ReplayInsts = []struct {
	GameID         string
	WhiteID        *int
	BlackID        *int
	Result         string
	Cause          string
	Mode           string
	WinEloDiff     float64
	LoseEloDiff    float64
	ReplayBlackElo float64
	ReplayWhiteElo float64
	PlayedOn       time.Time
	TurnCount      int32
}{
	// replay service tests (primarily the replay advanced search functionality)
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(1),
		BlackID:        ptr(2),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow,
		TurnCount:      42,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(2),
		BlackID:        ptr(3),
		Result:         "BLACK_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 900,
		PlayedOn:       TimeNow.Add(time.Hour * 24),
		TurnCount:      36,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(3),
		BlackID:        ptr(1),
		Result:         "DRAW",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     0,
		LoseEloDiff:    0,
		ReplayWhiteElo: 900,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow.Add(time.Hour * 24 * 2),
		TurnCount:      38,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(1),
		BlackID:        nil,
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     0,
		LoseEloDiff:    0,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow.Add(time.Hour * 24 * 3),
		TurnCount:      40,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(1),
		BlackID:        ptr(2),
		Result:         "BLACK_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow.Add(time.Hour * 24 * 4),
		TurnCount:      34,
	},
	// elo history tests
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(6),
		BlackID:        ptr(7),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 970,
		PlayedOn:       time.Date(1900, 1, 1, 1, 0, 0, 0, time.UTC),
		TurnCount:      25,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(6),
		BlackID:        ptr(7),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1060,
		ReplayBlackElo: 940,
		PlayedOn:       time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(6),
		BlackID:        ptr(7),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1090,
		ReplayBlackElo: 910,
		PlayedOn:       time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(6),
		BlackID:        ptr(7),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1120,
		ReplayBlackElo: 880,
		PlayedOn:       time.Date(2020, 1, 3, 1, 0, 0, 0, time.UTC),
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        ptr(6),
		BlackID:        ptr(7),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_1",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 1030,
		PlayedOn:       time.Date(2020, 1, 5, 1, 0, 0, 0, time.UTC),
	},
	// tournament matchmaking tests (required to mark games as finished)
	{
		GameID:         GameIDFinishedTournamentMatch1,
		WhiteID:        ptr(10),
		BlackID:        ptr(11),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1030,
		PlayedOn:       time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	},
	{
		GameID:         GameIDFinishedTournamentMatch2,
		WhiteID:        ptr(12),
		BlackID:        ptr(13),
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1030,
		PlayedOn:       time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	},
}

var GameIDFinishedTournamentMatch1 = uuid.NewString()
var GameIDFinishedTournamentMatch2 = uuid.NewString()

var ChallengeInsts = []struct {
	ChallengerID int64
	ChallengeeID int64
	Mode         string
	StartColor   string
	MadeOn       time.Time
}{
	{ChallengerID: 1, ChallengeeID: 2, Mode: "TIMED_3+2", StartColor: "RANDOM", MadeOn: TimeNow},
	{ChallengerID: 3, ChallengeeID: 1, Mode: "CORRESPONDENCE_1", StartColor: "RANDOM", MadeOn: TimeNow},
	{ChallengerID: 5, ChallengeeID: 2, Mode: "CORRESPONDENCE_1", StartColor: "RANDOM", MadeOn: TimeNow},
	{ChallengerID: 5, ChallengeeID: 4, Mode: "CORRESPONDENCE_1", StartColor: "RANDOM", MadeOn: TimeNow},
	// two challenges that are longer than the challenge max age away from "Now", to testing expiration
	{ChallengerID: 5, ChallengeeID: 3, Mode: "CORRESPONDENCE_1", StartColor: "RANDOM", MadeOn: TimeNow.Add(-1 * time.Hour * 24 * 365)},
	{ChallengerID: 5, ChallengeeID: 1, Mode: "CORRESPONDENCE_1", StartColor: "RANDOM", MadeOn: TimeNow.Add(-1 * time.Hour * 24 * 365)},
}

var (
	Tournament0LobbyKey                 = uuid.New()
	Tournament1LobbyFilledKey           = uuid.New()
	Tournament2ScheduledKnockoutKey     = uuid.New()
	Tournament3ScheduledRoundRobinKey   = uuid.New()
	Tournament4ScheduledSwissKey        = uuid.New()
	Tournament5InProgressKnockoutKey    = uuid.New()
	Tournament6InProgressRoundRobinKey  = uuid.New()
	Tournament7InProgressSwissKey       = uuid.New()
	Tournament8FinishedKey              = uuid.New()
	Tournament9InProgressUncompletedKey = uuid.New()
)

var TournamentInsts = []struct {
	TournamentKey uuid.UUID
	Name          string
	Rounds        int32
	Countdown     time.Duration
	CreatedOn     time.Time
	CreatedBy     int64
	Status        string
	Ruleset       string
	Mode          string
}{
	// LOBBY empty
	{
		TournamentKey: Tournament0LobbyKey,
		Name:          "Test Tournament 0",
		Rounds:        2,
		Countdown:     (5 * time.Minute) + (1 * time.Second),
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "LOBBY",
		Ruleset:       "KNOCKOUT",
		Mode:          "CORRESPONDENCE_1",
	},
	// LOBBY all participants filled
	{
		TournamentKey: Tournament1LobbyFilledKey,
		Name:          "Test Tournament 1",
		Rounds:        1,
		Countdown:     (5 * time.Minute) + (2 * time.Second),
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "LOBBY",
		Ruleset:       "KNOCKOUT",
		Mode:          "CORRESPONDENCE_7",
	},
	// SCHEDULED KNOCKOUT populated
	{
		TournamentKey: Tournament2ScheduledKnockoutKey,
		Name:          "Test Tournament 2",
		Rounds:        2,
		Countdown:     (5 * time.Minute) + (3 * time.Second),
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "SCHEDULED",
		Ruleset:       "KNOCKOUT",
		Mode:          "CORRESPONDENCE_1",
	},
	// SCHEDULED ROUND_ROBIN populated
	{
		TournamentKey: Tournament3ScheduledRoundRobinKey,
		Name:          "Test Tournament 3",
		Rounds:        4,
		Countdown:     10 * time.Minute,
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "SCHEDULED",
		Ruleset:       "ROUND_ROBIN",
		Mode:          "CORRESPONDENCE_1",
	},
	// SCHEDULED SWISS populated
	{
		TournamentKey: Tournament4ScheduledSwissKey,
		Name:          "Test Tournament 4",
		Rounds:        3,
		Countdown:     11 * time.Minute,
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "SCHEDULED",
		Ruleset:       "SWISS",
		Mode:          "CORRESPONDENCE_1",
	},
	// IN_PROGRESS KNOCKOUT populated
	{
		TournamentKey: Tournament5InProgressKnockoutKey,
		Name:          "Test Tournament 5",
		Rounds:        2,
		Countdown:     12 * time.Minute,
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "IN_PROGRESS",
		Ruleset:       "KNOCKOUT",
		Mode:          "CORRESPONDENCE_1",
	},
	// IN_PROGRESS ROUND_ROBIN populated
	{
		TournamentKey: Tournament6InProgressRoundRobinKey,
		Name:          "Test Tournament 6",
		Rounds:        4,
		Countdown:     (1 * time.Hour) + (1 * time.Minute) + (1 * time.Second),
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "IN_PROGRESS",
		Ruleset:       "ROUND_ROBIN",
		Mode:          "CORRESPONDENCE_1",
	},
	// IN_PROGRESS SWISS populated
	{
		TournamentKey: Tournament7InProgressSwissKey,
		Name:          "Test Tournament 7",
		Rounds:        3,
		Countdown:     (1 * time.Hour) + (2 * time.Minute) + (2 * time.Second) + (2 * time.Millisecond),
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "IN_PROGRESS",
		Ruleset:       "SWISS",
		Mode:          "CORRESPONDENCE_1",
	},
	// FINISHED all rounds populated
	{
		TournamentKey: Tournament8FinishedKey,
		Name:          "Test Tournament 8",
		Rounds:        1,
		Countdown:     5 * time.Minute,
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "FINISHED",
		Ruleset:       "SWISS",
		Mode:          "CORRESPONDENCE_1",
	},
	// IN_PROGRESS matches not finishd yet
	{
		TournamentKey: Tournament9InProgressUncompletedKey,
		Name:          "Test Tournament 9",
		Rounds:        1,
		Countdown:     1 * time.Minute,
		CreatedOn:     TimeNow,
		CreatedBy:     1,
		Status:        "IN_PROGRESS",
		Ruleset:       "SWISS",
		Mode:          "CORRESPONDENCE_1",
	},
}

// TournamentParticipantInsts JoinedOn must be deterministically ordered.
var TournamentParticipantInsts = []struct {
	TournamentKey uuid.UUID
	UserID        int64
	JoinedOn      time.Time
}{
	// LOBBY tournament (max participants)
	{
		TournamentKey: Tournament1LobbyFilledKey,
		UserID:        1,
		JoinedOn:      TimeNow.Add(time.Minute * 1),
	},
	{
		TournamentKey: Tournament1LobbyFilledKey,
		UserID:        2,
		JoinedOn:      TimeNow.Add(time.Minute * 2),
	},
	// SCHEDULED KNOCKOUT tournament
	{
		TournamentKey: Tournament2ScheduledKnockoutKey,
		UserID:        3,
		JoinedOn:      TimeNow.Add(time.Minute * 1),
	},
	{
		TournamentKey: Tournament2ScheduledKnockoutKey,
		UserID:        4,
		JoinedOn:      TimeNow.Add(time.Minute * 2),
	},
	{
		TournamentKey: Tournament2ScheduledKnockoutKey,
		UserID:        5,
		JoinedOn:      TimeNow.Add(time.Minute * 3),
	},
	{
		TournamentKey: Tournament2ScheduledKnockoutKey,
		UserID:        6,
		JoinedOn:      TimeNow.Add(time.Minute * 4),
	},
	// SCHEDULED ROUND_ROBIN tournament
	{
		TournamentKey: Tournament3ScheduledRoundRobinKey,
		UserID:        1,
		JoinedOn:      TimeNow.Add(time.Minute * 1),
	},
	{
		TournamentKey: Tournament3ScheduledRoundRobinKey,
		UserID:        2,
		JoinedOn:      TimeNow.Add(time.Minute * 2),
	},
	{
		TournamentKey: Tournament3ScheduledRoundRobinKey,
		UserID:        5,
		JoinedOn:      TimeNow.Add(time.Minute * 3),
	},
	{
		TournamentKey: Tournament3ScheduledRoundRobinKey,
		UserID:        6,
		JoinedOn:      TimeNow.Add(time.Minute * 4),
	},
	// SCHEDULED SWISS tournament
	{
		TournamentKey: Tournament4ScheduledSwissKey,
		UserID:        1,
		JoinedOn:      TimeNow.Add(time.Minute * 1),
	},
	{
		TournamentKey: Tournament4ScheduledSwissKey,
		UserID:        2,
		JoinedOn:      TimeNow.Add(time.Minute * 2),
	},
	{
		TournamentKey: Tournament4ScheduledSwissKey,
		UserID:        5,
		JoinedOn:      TimeNow.Add(time.Minute * 3),
	},
	{
		TournamentKey: Tournament4ScheduledSwissKey,
		UserID:        6,
		JoinedOn:      TimeNow.Add(time.Minute * 4),
	},
	// IN_PROGRESS tournament
	{
		TournamentKey: Tournament5InProgressKnockoutKey,
		UserID:        1,
		JoinedOn:      TimeNow.Add(time.Minute * 1),
	},
	{
		TournamentKey: Tournament5InProgressKnockoutKey,
		UserID:        2,
		JoinedOn:      TimeNow.Add(time.Minute * 2),
	},
	{
		TournamentKey: Tournament5InProgressKnockoutKey,
		UserID:        3,
		JoinedOn:      TimeNow.Add(time.Minute * 3),
	},
	{
		TournamentKey: Tournament5InProgressKnockoutKey,
		UserID:        4,
		JoinedOn:      TimeNow.Add(time.Minute * 4),
	},
	// FINISHED tournament participants (max participants)
	{
		TournamentKey: Tournament8FinishedKey,
		UserID:        1,
		JoinedOn:      TimeNow.Add(time.Minute * 5),
	},
	{
		TournamentKey: Tournament8FinishedKey,
		UserID:        2,
		JoinedOn:      TimeNow.Add(time.Minute * 6),
	},
}

var GameIDNotFinished = model.NewGameID() // does not exist in the replay table

// TournamentMatchInsts JoinedOn must be deterministically ordered.
var TournamentMatchInsts = []struct {
	GameID        string
	TournamentKey uuid.UUID
	Round         int32
	CreatedOn     time.Time
	WhiteID       int64
	BlackID       int64
}{
	// IN_PROGRESS tournament matches (Knockout, ready for next round)
	{
		GameID:        GameIDFinishedTournamentMatch1,
		TournamentKey: Tournament5InProgressKnockoutKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		WhiteID:       10,
		BlackID:       11,
	},
	{
		GameID:        GameIDFinishedTournamentMatch2,
		TournamentKey: Tournament5InProgressKnockoutKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 2),
		WhiteID:       12,
		BlackID:       13,
	},
	// IN_PROGRESS tournament matches (RoundRobin, ready for next round)
	{
		GameID:        GameIDFinishedTournamentMatch1,
		TournamentKey: Tournament6InProgressRoundRobinKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		WhiteID:       10,
		BlackID:       11,
	},
	{
		GameID:        GameIDFinishedTournamentMatch2,
		TournamentKey: Tournament6InProgressRoundRobinKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 2),
		WhiteID:       12,
		BlackID:       13,
	},
	// IN_PROGRESS tournament matches (Swiss, ready for next round)
	{
		GameID:        GameIDFinishedTournamentMatch1,
		TournamentKey: Tournament7InProgressSwissKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		WhiteID:       10,
		BlackID:       11,
	},
	{
		GameID:        GameIDFinishedTournamentMatch2,
		TournamentKey: Tournament7InProgressSwissKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 2),
		WhiteID:       12,
		BlackID:       13,
	},
	// FINISHED tournament matches (all matches)
	{
		GameID:        FirstReplayGameID,
		TournamentKey: Tournament8FinishedKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 3),
		WhiteID:       1,
		BlackID:       2,
	},
	// IN_PROGRESS tournament matches (not ready for next round)
	{
		GameID:        GameIDFinishedTournamentMatch1,
		TournamentKey: Tournament9InProgressUncompletedKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		WhiteID:       10,
		BlackID:       11,
	},
	{
		GameID:        GameIDNotFinished.String(), // does not exist in replays table, unfinished
		TournamentKey: Tournament9InProgressUncompletedKey,
		Round:         1,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		WhiteID:       10,
		BlackID:       11,
	},
}

var TestPbMoveHistory = func() *pb.MoveHistory {
	wantInitialGame := chess.NewEmptyGame(false)
	pbInitialGame := model.SerializeGame(&wantInitialGame)

	wantMoveReplay := &pb.MoveHistory{
		InitialGame: pbInitialGame,
		Steps: []*pb.HistMove{
			{Piece: 1, FromFile: 1, FromRank: 2, ToFile: 3, ToRank: 4, Notation: "pc5"},
		},
	}

	return wantMoveReplay
}()

var ReplayMoveHistories = []struct {
	replayID      int64
	pbMoveHistory *pb.MoveHistory
}{
	{
		replayID:      FirstReplayID,
		pbMoveHistory: TestPbMoveHistory,
	},
}

var TestEventID_TournamentCreation = uuid.New()
var TestEventID_TournamentCreation_GameID = model.NewGameID()

var Events = []struct {
	ID   string
	Data []byte
}{
	{
		ID:   TestEventID_TournamentCreation.String(),
		Data: []byte{},
		//Data: func() []byte {
		//	data, err := proto.Marshal(&pb.MatchCreations{
		//		Creations: []*pb.MatchCreation{
		//			{
		//				GameId:  TestEventID_TournamentCreation_GameID.String(),
		//				WhiteId: 1,
		//				BlackId: 2,
		//				Mode:    "CORRESPONDENCE_1",
		//			},
		//		},
		//	})
		//	if err != nil {
		//		panic(err)
		//	}
		//	return data
		//}(),
	},
}

var (
	GameID1 = model.NewGameID()
	GameID2 = model.NewGameID()
	GameID3 = model.NewGameID()
)

var GameMetas = []struct {
	Ordering int
	ID       string
	Mode     string
	WhiteID  int64
	BlackID  int64
}{
	{
		ID:      GameID1.String(),
		Mode:    "CORRESPONDENCE_1",
		BlackID: 2,
	},
	{
		ID:   GameID2.String(),
		Mode: "CORRESPONDENCE_1",
	},
	{
		ID:   GameID3.String(),
		Mode: "CORRESPONDENCE_1",
	},
}

func insertTestData(pool *pgxpool.Pool) error {
	ctx := context.WithValue(context.Background(), logutil.Trace, "insert-testing-data")

	batch := &pgx.Batch{}
	instCount := 0

	batchQueue := func(sql string, args ...any) {
		batch.Queue(sql, args...)
		instCount++
	}

	for _, inst := range UsersInsts {
		saltBytes := make([]byte, 16)
		if _, err := rand.Read(saltBytes); err != nil {
			return fmt.Errorf("failed to generate salt for user: %v", err)
		}
		salt := base64.StdEncoding.EncodeToString(saltBytes)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(inst.Password+salt), 12)
		if err != nil {
			return fmt.Errorf("failed to hash password for user: %v", err)
		}
		batchQueue(`
			INSERT INTO users (username, country, password, salt, joined_on)
			VALUES ($1, $2, $3, $4, $5)`,
			inst.Username,
			inst.Country,
			hashedPassword,
			salt,
			inst.JoinedOn,
		)
	}
	for _, inst := range UserModeElos {
		batchQueue(`
			INSERT INTO user_mode_elos (user_id, mode, elo, highest_elo, wins, losses) 
			VALUES ($1, $2, $3, $4, $5, $6);`,
			inst.UserID,
			inst.Mode,
			inst.Elo,
			inst.Elo,
			inst.Wins,
			inst.Losses,
		)
	}
	for _, inst := range ReplayInsts {
		batchQueue(`
			INSERT INTO replays (game_id, white_id, black_id, result, cause, win_elo_diff, lose_elo_diff, white_elo, black_elo, played_on, mode, turn_count) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`,
			inst.GameID,
			inst.WhiteID,
			inst.BlackID,
			inst.Result,
			inst.Cause,
			inst.WinEloDiff,
			inst.LoseEloDiff,
			inst.ReplayWhiteElo,
			inst.ReplayBlackElo,
			inst.PlayedOn,
			inst.Mode,
			inst.TurnCount,
		)
	}
	for _, inst := range ChallengeInsts {
		batchQueue("INSERT INTO challenges (challenger_id, challengee_id, mode, start_color, made_on) VALUES ($1, $2, $3, $4, $5);",
			inst.ChallengerID,
			inst.ChallengeeID,
			inst.Mode,
			inst.StartColor,
			inst.MadeOn,
		)
	}
	for _, inst := range TournamentInsts {
		batchQueue(`
			INSERT INTO tournaments (tournament_key, name, rounds, countdown, created_on, updated_on, created_by, status, ruleset, mode)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`,
			inst.TournamentKey,
			inst.Name,
			inst.Rounds,
			inst.Countdown.Milliseconds(),
			inst.CreatedOn,
			inst.CreatedOn,
			inst.CreatedBy,
			inst.Status,
			inst.Ruleset,
			inst.Mode,
		)
	}
	for _, inst := range TournamentParticipantInsts {
		batchQueue(`
			INSERT INTO tournament_participants (tournament_key, user_id, joined_on)
			VALUES ($1, $2, $3);`,
			inst.TournamentKey,
			inst.UserID,
			inst.JoinedOn,
		)
	}
	for _, inst := range TournamentMatchInsts {
		batchQueue(`
			INSERT INTO tournament_matches (game_id, tournament_key, round, created_on, white_id, black_id)
			VALUES ($1, $2, $3, $4, $5, $6);`,
			inst.GameID,
			inst.TournamentKey,
			inst.Round,
			inst.CreatedOn,
			inst.WhiteID,
			inst.BlackID,
		)
	}
	for _, inst := range ReplayMoveHistories {
		bytes, err := proto.Marshal(inst.pbMoveHistory)
		if err != nil {
			return fmt.Errorf("failed to marshal move history: %v", err)
		}
		batchQueue(`
			INSERT INTO replay_move_histories (replay_id, data) 
			VALUES ($1, $2);`,
			inst.replayID,
			bytes,
		)
	}
	for _, inst := range Events {
		batchQueue(`
			INSERT INTO event_keys (id, data) 
			VALUES ($1, $2);`,
			inst.ID,
			inst.Data,
		)
	}
	for _, inst := range GameMetas {
		batchQueue(`
			INSERT INTO games_metadata (game_id, mode,  white_id, black_id) 
			VALUES ($1, $2, $3, $4);`,
			inst.ID,
			inst.Mode,
			inst.WhiteID,
			inst.BlackID,
		)
	}

	batchResults := pool.SendBatch(ctx, batch)
	defer batchResults.Close()

	for range instCount {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to insert seed data: %v", err)
		}
	}
	return nil
}
