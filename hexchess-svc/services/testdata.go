package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"time"
)

// TestTimeNow a stable and consistent constant we use mock out the 'now' value in our test data
var TestTimeNow = time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)

var TestUsersInsts = []struct {
	Username string
	Password string
	Country  string
	JoinedOn time.Time
}{
	// used for user/challenge/replay tests
	{Username: "user1", Password: "password1", Country: "us", JoinedOn: TestTimeNow},
	{Username: "user2", Password: "password2", Country: "us", JoinedOn: TestTimeNow},
	{Username: "user3", Password: "password3", Country: "us", JoinedOn: TestTimeNow},
	{Username: "user4", Password: "password4", Country: "us", JoinedOn: TestTimeNow},
	{Username: "user5", Password: "password5", Country: "us", JoinedOn: TestTimeNow},
	// used for elo histories tests.
	{Username: "user6", Password: "password6", Country: "us", JoinedOn: TestTimeNow},
	{Username: "user7", Password: "password7", Country: "us", JoinedOn: TestTimeNow},
}

var TestUserEntities = []UserEntity{
	{
		ID:       1,
		Username: "user1",
		Country:  "us",
		Bio:      "",
		JoinedOn: TestTimeNow.Local(),
	},
	{
		ID:       2,
		Username: "user2",
		Country:  "us",
		Bio:      "",
		JoinedOn: TestTimeNow.Local(),
	},
}

var TestUserModeElos = []struct {
	UserID int64
	Mode   GameMode
	Elo    int64
	Wins   int32
	Losses int32
}{
	{UserID: 1, Mode: ModeCorrespondence7, Elo: 1000, Wins: 2, Losses: 2},
	{UserID: 1, Mode: ModeTimed3Plus2, Elo: 1020, Wins: 5, Losses: 4},
	{UserID: 1, Mode: ModeTimed15Plus10, Elo: 1030, Wins: 4, Losses: 3},
	{UserID: 1, Mode: ModeTimed1Plus0, Elo: 1000, Wins: 5, Losses: 5},
	{UserID: 2, Mode: ModeTimed1Plus0, Elo: 1000},
	{UserID: 3, Mode: ModeCorrespondence7, Elo: 900},
	{UserID: 3, Mode: ModeCorrespondence1, Elo: 900},
	{UserID: 4, Mode: ModeCorrespondence1, Elo: 2000},
	{UserID: 5, Mode: ModeCorrespondence1, Elo: 1500},
}

var TestUserStats = []UserStatsEntity{
	{
		TotalWins:    16,
		TotalLosses:  14,
		AvgElo:       1012.5,
		HighestElo:   1030,
		TotalWinrate: 52,
		ModeStats: []ModeStatsEntity{
			{Mode: ModeCorrespondence7, Rank: 1, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
			{Mode: ModeTimed3Plus2, Rank: 1, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
			{Mode: ModeTimed15Plus10, Rank: 1, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
			{Mode: ModeTimed1Plus0, Rank: 1, Wins: 5, Losses: 5, Winrate: 50, Elo: 1000, HighestElo: 1000},
		},
	},
}

var TestReplayInsts = []struct {
	WhiteID        int64
	BlackID        int64
	Result         ReplayResult
	Cause          ReplayCause
	Mode           GameMode
	WinEloDiff     float64
	LoseEloDiff    float64
	ReplayBlackElo float64
	ReplayWhiteElo float64
	PlayedOn       time.Time
}{
	// used for testing individual replays
	{
		WhiteID:        1,
		BlackID:        2,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
		PlayedOn:       TestTimeNow,
	},
	{
		WhiteID:        2,
		BlackID:        3,
		Result:         BlackWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 900,
		PlayedOn:       TestTimeNow,
	},
	{
		WhiteID:        3,
		BlackID:        1,
		Result:         Draw,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     0,
		LoseEloDiff:    0,
		ReplayWhiteElo: 900,
		ReplayBlackElo: 1000,
		PlayedOn:       TestTimeNow,
	},

	// used for testing elo histories
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 970,
		PlayedOn:       time.Date(1900, 1, 1, 1, 0, 0, 0, time.UTC),
	},
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1060,
		ReplayBlackElo: 940,
		PlayedOn:       time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	},
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1090,
		ReplayBlackElo: 910,
		PlayedOn:       time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
	},
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence7,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1120,
		ReplayBlackElo: 880,
		PlayedOn:       time.Date(2020, 1, 3, 1, 0, 0, 0, time.UTC),
	},
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeCorrespondence1,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: -1030,
		PlayedOn:       time.Date(2020, 1, 5, 1, 0, 0, 0, time.UTC),
	},
}

var TestReplayEntities = []ReplayEntity{
	{
		ID:           3,
		WhiteID:      3,
		BlackID:      1,
		WhiteName:    "user3",
		BlackName:    "user1",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         ModeCorrespondence7,
		Result:       Draw,
		Cause:        Checkmate,
		WinEloDiff:   0,
		LoseEloDiff:  0,
		WhiteElo:     900,
		BlackElo:     1000,
		WhiteEloDiff: 0,
		BlackEloDiff: 0,
		PlayedOn:     TestTimeNow.Local(),
	},
	{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         ModeCorrespondence7,
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinEloDiff:   30,
		LoseEloDiff:  -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
		PlayedOn:     TestTimeNow.Local(),
	},
}

var TestChallengeInsts = []struct {
	ChallengerID int64
	ChallengeeID int64
	Mode         GameMode
	StartColor   ColorSelect
	MadeOn       time.Time
}{
	{ChallengerID: 1, ChallengeeID: 2, Mode: ModeTimed3Plus2, StartColor: ColorRandom, MadeOn: TestTimeNow},
	{ChallengerID: 3, ChallengeeID: 1, Mode: ModeCorrespondence1, StartColor: ColorRandom, MadeOn: TestTimeNow},
	{ChallengerID: 5, ChallengeeID: 2, Mode: ModeCorrespondence1, StartColor: ColorRandom, MadeOn: time.Unix(20500, 0)},
	{ChallengerID: 5, ChallengeeID: 4, Mode: ModeCorrespondence1, StartColor: ColorRandom, MadeOn: time.Unix(19500, 0)},
	{ChallengerID: 5, ChallengeeID: 3, Mode: ModeCorrespondence1, StartColor: ColorRandom, MadeOn: time.Unix(0, 0)},
	{ChallengerID: 5, ChallengeeID: 1, Mode: ModeCorrespondence1, StartColor: ColorRandom, MadeOn: time.Unix(0, 0)},
}

var TestChallengeEntities = []ChallengeEntity{
	{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1020,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeTimed3Plus2,
		StartColor:        ColorRandom,
		MadeOn:            TestTimeNow.Local(),
		ExpiresOn:         TestTimeNow.Local().Add(ExpireChallengeThreshold),
	},
	{
		ChallengerID:      3,
		ChallengerName:    "user3",
		ChallengerCountry: "us",
		ChallengerElo:     900,
		ChallengeeID:      1,
		ChallengeeName:    "user1",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeCorrespondence1,
		StartColor:        ColorRandom,
		MadeOn:            TestTimeNow.Local(),
		ExpiresOn:         TestTimeNow.Local().Add(ExpireChallengeThreshold),
	},
	{
		ChallengerID:      5,
		ChallengerName:    "user5",
		ChallengerCountry: "us",
		ChallengerElo:     1500,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeCorrespondence1,
		StartColor:        ColorRandom,
		MadeOn:            time.Unix(20500, 0),
		ExpiresOn:         time.Unix(20500, 0).Add(ExpireChallengeThreshold),
	},
	{
		ChallengerID:      5,
		ChallengerName:    "user5",
		ChallengerCountry: "us",
		ChallengerElo:     1500,
		ChallengeeID:      4,
		ChallengeeName:    "user4",
		ChallengeeCountry: "us",
		ChallengeeElo:     2000,
		Mode:              ModeCorrespondence1,
		StartColor:        ColorRandom,
		MadeOn:            time.Unix(19500, 0),
		ExpiresOn:         time.Unix(19500, 0).Add(ExpireChallengeThreshold),
	},
}

func InsertTestData(t db.TestLogger, pool *pgxpool.Pool) {
	ctx := context.WithValue(context.Background(), logutil.Trace, "insert-test-data")

	batch := &pgx.Batch{}

	instCount := len(TestUsersInsts) + len(TestUserModeElos) + len(TestReplayInsts) + len(TestChallengeInsts)

	for _, inst := range TestUsersInsts {
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
	for _, inst := range TestUserModeElos {
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
	for _, inst := range TestReplayInsts {
		batch.Queue(`
			INSERT INTO replays (white_id, black_id, result, cause, win_elo_diff, lose_elo_diff, white_elo, black_elo, played_on, mode, move_history) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
			`,
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
			[]byte{},
		)
	}
	for _, inst := range TestChallengeInsts {
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
