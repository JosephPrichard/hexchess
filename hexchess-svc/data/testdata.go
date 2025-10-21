package data

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/logs"
	"time"
)

var TestUsersInsts = []UserInst{
	{Username: "user1", Password: "password1", Country: "us", Elo: 1000},
	{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1},
	{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8},
	{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20},
	{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35},
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
		Winrate:    0,
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
		Winrate:    100,
	},
}

func createTestUser(t TestLogger, q *db.Queries, inst UserInst) UserEntity {
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-test-user")
	u, err := InsertUser(ctx, q, inst)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	return u
}

func createTestUsers(t TestLogger, q *db.Queries, insts ...UserInst) []UserEntity {
	var users []UserEntity
	for _, inst := range insts {
		users = append(users, createTestUser(t, q, inst))
	}
	return users
}

var TestReplayInsts = []ReplayInst{
	{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "[]"},
	{2, 3, int32(BlackWin), int32(Checkmate), 30, -30, "{}"},
	{3, 1, int32(Draw), int32(Checkmate), 30, -30, "{}"},
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
		WinElo:       30,
		LoseElo:      -30,
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
		WinElo:       30,
		LoseElo:      -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
	},
}

func createTestReplays(t TestLogger, q *db.Queries, insts ...ReplayInst) {
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-test-replays")
	for _, inst := range insts {
		_, err := InsertReplay(ctx, q, inst)
		if err != nil {
			t.Fatalf("failed to insert test replay: %v", err)
		}
	}
}

var TestChallengeInsts = []ChallengeInst{
	{5, 2, Unlimited, Random, time.Unix(20500, 0)},
	{5, 4, Unlimited, Random, time.Unix(19500, 0)},
	{5, 3, Unlimited, Random, time.Unix(0, 0)},
	{5, 1, Unlimited, Random, time.Unix(0, 0)},
	{1, 2, Unlimited, Random, time.Now()},
	{3, 1, Unlimited, Random, time.Now()},
}

func createTestChallenges(t TestLogger, q *db.Queries, insts ...ChallengeInst) {
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-test-challenges")
	for _, c := range insts {
		err := InsertChallenge(ctx, q, c)
		if err != nil {
			t.Fatalf("failed to insert test challenges: %v", err)
		}
	}
}

func CreateTestData(t TestLogger, q *db.Queries) {
	createTestUsers(t, q, TestUsersInsts...)
	createTestReplays(t, q, TestReplayInsts...)
	createTestChallenges(t, q, TestChallengeInsts...)
}
