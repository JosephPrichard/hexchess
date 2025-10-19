package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/logs"
	"testing"
	"time"
)

func TestInsertThenVerify(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-insert-then-verify")

	user1 := "user1-" + uuid.NewString()
	user2 := "user2-" + uuid.NewString()
	user3 := "user3-" + uuid.NewString()

	u1, err := InsertUser(ctx, pgDB.Q, UserInst{Username: user1, Password: "password1"})
	assert.NoError(t, err)
	u2, err := InsertUser(ctx, pgDB.Q, UserInst{Username: user2, Password: "password2"})
	assert.NoError(t, err)
	u3, err := InsertUser(ctx, pgDB.Q, UserInst{Username: user3, Password: "password3"})
	assert.NoError(t, err)

	v1, err := VerifyUser(ctx, pgDB.Q, user1, "password1")
	assert.NoError(t, err)
	v2, err := VerifyUser(ctx, pgDB.Q, user2, "password2")
	assert.NoError(t, err)
	_, err1 := VerifyUser(ctx, pgDB.Q, user2, "wrong-password")
	v4, err := VerifyUser(ctx, pgDB.Q, user3, "password3")
	assert.NoError(t, err)
	_, err2 := VerifyUser(ctx, pgDB.Q, user1, "password3")

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
	assert.Error(t, err1)
	assert.Equal(t, u3.ID, v4.ID)
	assert.Error(t, err2)
}

func TestBatchInsertThenGet(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-batch-insert-then-get")

	insts := []UserInst{
		{Username: "user1-" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1005, Wins: 10, Losses: 10},
		{Username: "user2-" + uuid.NewString(), Password: "password2", Country: "eu", Elo: 1035, Wins: 12, Losses: 0},
	}
	users, err := BatchInsertUsers(ctx, pgDB.Q, insts)

	for i := range users {
		users[i].ID = 0
		users[i].JoinedOn = time.Time{}
	}
	expUsers := []UserEntity{
		{Username: insts[0].Username, Country: "us", Elo: 1005, HighestElo: 1005, Wins: 10, Losses: 10, Total: 20, Winrate: 50.0},
		{Username: insts[1].Username, Country: "eu", Elo: 1035, HighestElo: 1035, Wins: 12, Losses: 0, Total: 12, Winrate: 100.0},
	}

	assert.Equal(t, expUsers, users)
	assert.NoError(t, err)
}

func TestUpdateUser(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	tests := []struct {
		inst        UserInst
		udpt        UpdtUserParams
		expUsername string
		expBio      string
		expCountry  string
	}{
		{
			inst:        UserInst{Username: "user1" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000},
			udpt:        UpdtUserParams{Username: "user1-changed", Bio: "Testing123"},
			expUsername: "user1-changed",
			expBio:      "Testing123",
			expCountry:  "us",
		},
		{
			inst:        UserInst{Username: "user2" + uuid.NewString(), Password: "password2", Country: "us", Elo: 1000},
			udpt:        UpdtUserParams{Username: "user2-changed", Country: "eu"},
			expUsername: "user2-changed",
			expBio:      "",
			expCountry:  "eu",
		},
	}

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-update-user")

	for _, test := range tests {
		testUser := createTestUser(t, pgDB.Q, test.inst)

		assert.NoError(t, UpdateUser(ctx, pgDB.Q, testUser.ID, test.udpt))

		u, err := GetUserById(ctx, pgDB.Q, testUser.ID)
		assert.NoError(t, err)

		assert.Equal(t, test.expUsername, u.Username)
		assert.Equal(t, test.expBio, u.Bio)
		assert.Equal(t, test.expCountry, u.Country)
	}
}

func TestUpdatePassword(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	testUser := createTestUser(t, pgDB.Q, UserInst{Username: "user1-" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000})

	ctx := context.WithValue(context.Background(), logs.TraceKey, "update-password")

	assert.NoError(t, UpdateUserPassword(ctx, pgDB.Q, testUser.ID, "password-new"))

	u1, err := GetUserById(ctx, pgDB.Q, testUser.ID)
	assert.NoError(t, err)
	v1, err := VerifyUser(ctx, pgDB.Q, testUser.Username, "password-new")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func TestSearchByName(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	createTestUsers(t, pgDB.Q, UserInst{Username: "johnny", Password: "password6"}, UserInst{Username: "john", Password: "password7"})

	ctx := context.WithValue(context.Background(), logs.TraceKey, "search-by-name")

	list, err := SearchUsersByName(ctx, pgDB.Q, "john", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}
