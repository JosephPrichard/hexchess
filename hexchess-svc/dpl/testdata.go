package dpl

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"time"
)

var TestUsersInsts = []UserInst{
	// used for user/challenge/replay tests
	{Username: "user1", Password: "password1", Country: "us", Elo: 1000},
	{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1},
	{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8},
	{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20},
	{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35},
	// used for elo histories tests.
	{Username: "user6", Password: "password6", Country: "us", Elo: 1090, Wins: 3, Losses: 0},
	{Username: "user7", Password: "password7", Country: "us", Elo: 910, Wins: 0, Losses: 3},
}

var TestUserEntities = []UserEntity{
	{
		ID:         1,
		Username:   "user1",
		Country:    "us",
		Elo:        1000,
		HighestElo: 1000,
		Wins:       0,
		Losses:     0,
		Rank:       1,
		Bio:        "",
		Total:      0,
		WinRate:    0,
	},
	{
		ID:         2,
		Username:   "user2",
		Country:    "us",
		Elo:        1000,
		HighestElo: 1000,
		Wins:       1,
		Losses:     0,
		Rank:       2,
		Bio:        "",
		Total:      1,
		WinRate:    100,
	},
}

var LastUserID = int64(len(TestUsersInsts))

func createTestUser(t infra.TestLogger, query *db.Queries, inst UserInst) UserEntity {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-user")
	u, err := InsertUser(ctx, query, inst)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return u
}

func insertTestUsers(t infra.TestLogger, query *db.Queries, insts ...UserInst) []UserEntity {
	var users []UserEntity
	for _, inst := range insts {
		users = append(users, createTestUser(t, query, inst))
	}
	return users
}

var TestReplayInsts = []ReplayInst{
	// used for testing individual replays
	{
		WhiteID:        1,
		BlackID:        2,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeUnlimited,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1000,
		ReplayBlackElo: 1000,
	},
	{
		WhiteID:        2,
		BlackID:        3,
		Result:         BlackWin,
		Cause:          Checkmate,
		Mode:           ModeUnlimited,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1030,
		ReplayBlackElo: 900,
	},
	{
		WhiteID:        3,
		BlackID:        1,
		Result:         Draw,
		Cause:          Checkmate,
		Mode:           ModeUnlimited,
		WinEloDiff:     0,
		LoseEloDiff:    0,
		ReplayWhiteElo: 900,
		ReplayBlackElo: 1000,
	},

	// used for testing elo histories
	{
		WhiteID:        6,
		BlackID:        7,
		Result:         WhiteWin,
		Cause:          Checkmate,
		Mode:           ModeUnlimited,
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
		Mode:           ModeRealTime,
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
		Mode:           ModeRealTime,
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
		Mode:           ModeUnlimited,
		WinEloDiff:     30,
		LoseEloDiff:    -30,
		ReplayWhiteElo: 1120,
		ReplayBlackElo: 880,
		PlayedOn:       time.Date(2020, 1, 3, 1, 0, 0, 0, time.UTC),
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
		Result:       Draw,
		Cause:        Checkmate,
		WinEloDiff:   0,
		LoseEloDiff:  0,
		WhiteElo:     900,
		BlackElo:     1000,
		WhiteEloDiff: 0,
		BlackEloDiff: 0,
	},
	{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinEloDiff:   30,
		LoseEloDiff:  -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
	},
}

func insertTestReplays(t infra.TestLogger, query *db.Queries, insts ...ReplayInst) {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-replays")
	for _, inst := range insts {
		if _, err := InsertReplay(ctx, query, inst); err != nil {
			t.Fatalf("insert test replay: %v", err)
		}
	}
}

var TestChallengeInsts = []ChallengeInst{
	{ChallengerID: 1, ChallengeeID: 2, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Now()},
	{ChallengerID: 3, ChallengeeID: 1, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Now()},
	{ChallengerID: 5, ChallengeeID: 2, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Unix(20500, 0)},
	{ChallengerID: 5, ChallengeeID: 4, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Unix(19500, 0)},
	{ChallengerID: 5, ChallengeeID: 3, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Unix(0, 0)},
	{ChallengerID: 5, ChallengeeID: 1, TimeControl: TcUnlimited, StartColor: CsRandom, MadeOn: time.Unix(0, 0)},
}

var TestChallengeEntities = []ChallengeEntity{
	{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		TimeControl:       TcUnlimited,
		StartColor:        CsRandom,
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
		TimeControl:       TcUnlimited,
		StartColor:        CsRandom,
	},
}

func insertTestChallenges(t infra.TestLogger, query *db.Queries, insts ...ChallengeInst) {
	ctx := context.WithValue(context.Background(), util.Trace, "create-test-challenges")
	for _, c := range insts {
		if err := InsertChallenge(ctx, query, c); err != nil {
			t.Fatalf("insert test challenges: %v", err)
		}
	}
}

func InsertTestData(t infra.TestLogger, query *db.Queries) {
	insertTestUsers(t, query, TestUsersInsts...)
	insertTestReplays(t, query, TestReplayInsts...)
	insertTestChallenges(t, query, TestChallengeInsts...)
}
