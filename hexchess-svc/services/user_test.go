package svc

import (
	"context"
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

	ctx := context.WithValue(t.Context(), util.Trace, "testing-insert-then-verify")

	user1 := "user1-test"

	// when
	u1, err := InsertUser(ctx, pdb.Query, UserInst{Username: user1, Password: "password1", Country: "us", JoinedOn: TestTimeNow})
	assert.NoError(t, err)

	v1, err := verifyUser(ctx, pdb.Query, user1, "password1")
	assert.NoError(t, err)

	dbU1, err := GetUserByID(ctx, pdb.Query, LastUserID+1)
	assert.NoError(t, err)

	var attemptsErrs []error
	for range LoginAttemptsDivisor {
		_, err := verifyUser(ctx, pdb.Query, user1, "wrong-password")
		attemptsErrs = append(attemptsErrs, err)
	}
	_, errTooMany := verifyUser(ctx, pdb.Query, user1, "wrong-password")

	// then
	var wantAttemptErrs []error
	for range LoginAttemptsDivisor {
		wantAttemptErrs = append(wantAttemptErrs, ErrUserNotFound)
	}
	assert.Equal(t, wantAttemptErrs, attemptsErrs)
	assert.Equal(t, ErrTooManyLoginAttempts, errTooMany)

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, UserEntity{
		ID:       LastUserID + 1,
		Username: "user1-test",
		Country:  "us",
		JoinedOn: TestTimeNow.Local(),
	}, dbU1)
}

func TestBatchInsertThenGet(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-batch-insert-then-get")

	// when
	insts := []BatchUserInst{
		{Username: "user1-test", Password: "password1", Country: "us", Elo: 1005},
		{Username: "user2-test", Password: "password2", Country: "eu", Elo: 1035},
	}
	users, batchErr := BatchInsertUsers(ctx, pdb.Query, insts)

	for i := range users {
		users[i].ID = 0
		users[i].JoinedOn = time.Time{}
	}
	wantUsers := []UserEntity{
		{Username: insts[0].Username, Country: "us"},
		{Username: insts[1].Username, Country: "eu"},
	}

	// then
	assert.Equal(t, wantUsers, users)
	assert.NoError(t, batchErr)
}

func TestUpdateUser(t *testing.T) {
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-update-user")

	for _, test := range []struct {
		userID       int64
		udpt         UpdtUserParams
		wantUsername string
		wantBio      string
		wantCountry  string
	}{
		{
			userID:       1,
			udpt:         UpdtUserParams{Username: "user1-changed", Bio: "Testing123"},
			wantUsername: "user1-changed",
			wantBio:      "Testing123",
			wantCountry:  "us",
		},
		{
			userID:       2,
			udpt:         UpdtUserParams{Username: "user2-changed", Country: "eu"},
			wantUsername: "user2-changed",
			wantBio:      "",
			wantCountry:  "eu",
		},
	} {
		_, err := UpdateUser(ctx, pdb.Query, test.userID, test.udpt)
		assert.NoError(t, err)

		u, err := GetUserByID(ctx, pdb.Query, test.userID)
		assert.NoError(t, err)

		assert.Equal(t, test.wantUsername, u.Username)
		assert.Equal(t, test.wantBio, u.Bio)
		assert.Equal(t, test.wantCountry, u.Country)
	}
}

func TestSelectOrInsertGoogleUser(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-insert-google-user")

	testAccountID := "test-account-id"

	inst := GoogleUserInst{Username: "username", Country: "us", JoinedOn: TestTimeNow}

	// when
	u1, err := SelectOrInsertGoogleUser(ctx, pdb.Query, testAccountID, inst)
	assert.NoError(t, err)

	u2, err := SelectOrInsertGoogleUser(ctx, pdb.Query, testAccountID, inst)
	assert.NoError(t, err)

	dbU1, err := GetUserByID(ctx, pdb.Query, u1.ID)
	assert.NoError(t, err)

	// then
	verifiedUser := VerifiedUser{ID: LastUserID + 1, Username: "username", Country: "us"}
	assert.Equal(t, verifiedUser, u1)
	assert.Equal(t, verifiedUser, u2)

	assert.Equal(t, UserEntity{
		ID:       LastUserID + 1,
		Username: "username",
		Country:  "us",
		JoinedOn: TestTimeNow.Local(),
	}, dbU1)
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "update-password")

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

	ctx := context.WithValue(t.Context(), util.Trace, "search-by-name")

	// when
	_, err := BatchInsertUsers(ctx, pdb.Query, []BatchUserInst{{Username: "john", Password: "password5"}, {Username: "johnny", Password: "password5"}})
	assert.NoError(t, err)

	list, err := SearchUsersByName(ctx, pdb.Query, "john", 1, 20)
	assert.NoError(t, err)

	// then
	assert.Len(t, list, 2)
}
