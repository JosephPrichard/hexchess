package itest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/google/uuid"
	"time"

	"hexchess-svc/pkg/logutil"

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
	// used for user/challenge/replay tests
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
}

var UserModeElos = []struct {
	UserID int64
	Mode   string
	Elo    int64
	Wins   int32
	Losses int32
}{
	{UserID: 1, Mode: "CORRESPONDENCE_7", Elo: 1000, Wins: 2, Losses: 2},
	{UserID: 1, Mode: "TIMED_3+2", Elo: 1020, Wins: 5, Losses: 4},
	{UserID: 1, Mode: "TIMED_15+10", Elo: 1030, Wins: 4, Losses: 3},
	{UserID: 1, Mode: "TIMED_1+0", Elo: 1050, Wins: 6, Losses: 5},
	{UserID: 2, Mode: "TIMED_1+0", Elo: 1000},
	{UserID: 3, Mode: "CORRESPONDENCE_7", Elo: 900},
	{UserID: 3, Mode: "CORRESPONDENCE_1", Elo: 900},
	{UserID: 4, Mode: "CORRESPONDENCE_1", Elo: 2000},
	{UserID: 5, Mode: "CORRESPONDENCE_1", Elo: 1500},
	{UserID: 8, Mode: "CORRESPONDENCE_1", Elo: 1000, Wins: 2, Losses: 2},
	{UserID: 8, Mode: "CORRESPONDENCE_7", Elo: 2000, Wins: 10, Losses: 2},
	{UserID: 9, Mode: "CORRESPONDENCE_1", Elo: 1500, Wins: 5, Losses: 2},
}

var ConstantGameID = uuid.NewString()

var ReplayInsts = []struct {
	GameID         string
	WhiteID        int64
	BlackID        int64
	Result         string
	Cause          string
	Mode           string
	WinEloDiff     float64
	LoseEloDiff    float64
	ReplayBlackElo float64
	ReplayWhiteElo float64
	PlayedOn       time.Time
}{
	{
		GameID:         ConstantGameID,
		WhiteID:        1,
		BlackID:        2,
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        2,
		BlackID:        3,
		Result:         "BLACK_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 900,
		PlayedOn:       TimeNow,
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        3,
		BlackID:        1,
		Result:         "DRAW",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     0,
		LoseEloDiff:    0,
		ReplayWhiteElo: 900,
		ReplayBlackElo: 1000,
		PlayedOn:       TimeNow,
	},

	// elo history tests
	{
		GameID:         uuid.NewString(),
		WhiteID:        6,
		BlackID:        7,
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_7",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 970,
		PlayedOn:       time.Date(1900, 1, 1, 1, 0, 0, 0, time.UTC),
	},
	{
		GameID:         uuid.NewString(),
		WhiteID:        6,
		BlackID:        7,
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
		WhiteID:        6,
		BlackID:        7,
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
		WhiteID:        6,
		BlackID:        7,
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
		WhiteID:        6,
		BlackID:        7,
		Result:         "WHITE_WINS",
		Cause:          "CHECKMATE",
		Mode:           "CORRESPONDENCE_1",
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: -1030,
		PlayedOn:       time.Date(2020, 1, 5, 1, 0, 0, 0, time.UTC),
	},
}

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

func insertTestData(t logutil.TestLogger, pool *pgxpool.Pool) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "insert-testing-data")

	batch := &pgx.Batch{}

	instCount := len(UsersInsts) + len(UserModeElos) + len(ReplayInsts) + len(ChallengeInsts)

	for _, inst := range UsersInsts {
		saltBytes := make([]byte, 16)
		if _, err := rand.Read(saltBytes); err != nil {
			t.Fatalf("failed to generate salt for user: %v", err)
		}
		salt := base64.StdEncoding.EncodeToString(saltBytes)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(inst.Password+salt), 12)
		if err != nil {
			t.Fatalf("failed to hash password for user: %v", err)
		}
		batch.Queue(`
			INSERT INTO users (username, country, password, salt, joined_on)
			VALUES ($1, $2, $3, $4, $5)
			`,
			inst.Username,
			inst.Country,
			hashedPassword,
			salt,
			inst.JoinedOn,
		)
	}
	for _, inst := range UserModeElos {
		batch.Queue(`
			INSERT INTO user_mode_elos (user_id, mode, elo, highest_elo, wins, losses) 
			VALUES ($1, $2, $3, $4, $5, $6);
			`,
			inst.UserID,
			inst.Mode,
			inst.Elo,
			inst.Elo,
			inst.Wins,
			inst.Losses,
		)
	}
	for _, inst := range ReplayInsts {
		batch.Queue(`
			INSERT INTO replays (game_id, white_id, black_id, result, cause, win_elo_diff, lose_elo_diff, white_elo, black_elo, played_on, mode) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
			`,
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
		)
	}
	for _, inst := range ChallengeInsts {
		batch.Queue("INSERT INTO challenges (challenger_id, challengee_id, mode, start_color, made_on) VALUES ($1, $2, $3, $4, $5);",
			inst.ChallengerID,
			inst.ChallengeeID,
			inst.Mode,
			inst.StartColor,
			inst.MadeOn,
		)
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	for range instCount {
		if _, err := br.Exec(); err != nil {
			t.Fatalf("failed to insert seed data: %v", err)
		}
	}
}
