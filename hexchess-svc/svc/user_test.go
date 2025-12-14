package svc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestInsertThenVerify(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-insert-then-verify")

	user1 := "user1-test"
	user2 := "user2-test"

	// when
	u1, err := InsertUser(ctx, pdb.Query, UserInst{Username: user1, Password: "password1"})
	assert.NoError(t, err)
	u2, err := InsertUser(ctx, pdb.Query, UserInst{Username: user2, Password: "password2"})
	assert.NoError(t, err)

	v1, err := VerifyUserTx(ctx, pdb, user1, "password1")
	assert.NoError(t, err)
	v2, err := VerifyUserTx(ctx, pdb, user2, "password2")
	assert.NoError(t, err)

	for range LoginAttemptsDivisor {
		_, err := VerifyUserTx(ctx, pdb, user2, "wrong-password")
		assert.Equal(t, ErrUserNotFound, err)
	}
	_, err = VerifyUserTx(ctx, pdb, user2, "wrong-password")
	assert.Equal(t, ErrTooManyLoginAttempts, err)

	// then
	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
}

func TestBatchInsertThenGet(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-batch-insert-then-get")

	// when
	insts := []UserInst{
		{Username: "user1-test", Password: "password1", Country: "us", Elo: 1005, Wins: 10, Losses: 10},
		{Username: "user2-test", Password: "password2", Country: "eu", Elo: 1035, Wins: 12, Losses: 0},
	}
	users, err := BatchInsertUsers(ctx, pdb.Query, insts)

	for i := range users {
		users[i].ID = 0
		users[i].JoinedOn = time.Time{}
	}
	expUsers := []UserEntity{
		{Username: insts[0].Username, Country: "us", Elo: 1005, HighestElo: 1005, Wins: 10, Losses: 10, Total: 20, WinRate: 50.0},
		{Username: insts[1].Username, Country: "eu", Elo: 1035, HighestElo: 1035, Wins: 12, Losses: 0, Total: 12, WinRate: 100.0},
	}

	// then
	assert.Equal(t, expUsers, users)
	assert.NoError(t, err)
}

func TestInsertAndUpdateUser(t *testing.T) {
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-update-user")

	for _, test := range []struct {
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
	} {
		testUser, err := InsertUser(ctx, pdb.Query, test.inst)
		assert.NoError(t, err)

		_, err = UpdateUser(ctx, pdb.Query, testUser.ID, test.udpt)
		assert.NoError(t, err)

		u, err := GetUserByID(ctx, pdb.Query, testUser.ID)
		assert.NoError(t, err)

		assert.Equal(t, test.expUsername, u.Username)
		assert.Equal(t, test.expBio, u.Bio)
		assert.Equal(t, test.expCountry, u.Country)
	}
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "update-password")

	// when
	err := UpdateUserPassword(ctx, pdb.Query, TestUserEntities[0].ID, "password-new")
	assert.NoError(t, err)

	u1, err := GetUserByID(ctx, pdb.Query, TestUserEntities[0].ID)
	assert.NoError(t, err)
	v1, err := verifyUser(ctx, pdb.Query, TestUserEntities[0].Username, "password-new")
	assert.NoError(t, err)

	// then
	assert.Equal(t, u1.ID, v1.ID)
}

func TestInsertThenSearchByName(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "search-by-name")

	// when
	for _, inst := range []UserInst{{Username: "johnny", Password: "password6"}, {Username: "john", Password: "password7"}} {
		_, err := InsertUser(ctx, pdb.Query, inst)
		assert.NoError(t, err)
	}
	list, err := SearchUsersByName(ctx, pdb.Query, "john", 1, 20)
	assert.NoError(t, err)

	// then
	assert.Len(t, list, 2)
}
