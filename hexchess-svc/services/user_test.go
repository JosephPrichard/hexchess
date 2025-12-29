package svc

import (
	"context"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

var testUserCmptOpts = cmpopts.IgnoreFields(UserEntity{}, "ID")
var testVerifiedUserCmptOpts = cmpopts.IgnoreFields(VerifiedUser{}, "ID")

func TestInsertThenVerify(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-insert-then-verify")

	user1 := "user1-test"

	// when
	u1, err := InsertUser(ctx, pdb.Query, UserInst{Username: user1, Password: "password1", Country: "us", JoinedOn: TestTimeNow})
	require.NoError(t, err)

	v1, err := verifyUser(ctx, pdb.Query, user1, "password1")
	require.NoError(t, err)

	dbU1, err := GetUserByID(ctx, pdb.Query, v1.ID)
	require.NoError(t, err)

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
	wantU1 := UserEntity{
		Username: "user1-test",
		Country:  "us",
		JoinedOn: TestTimeNow.Local(),
	}
	assertutil.AssertEqualIgnoring(t, wantU1, dbU1, testUserCmptOpts)
}

func TestBatchInsertThenGet(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-batch-insert-then-get")

	// when
	insts := []UserInst{
		{Username: "user1-test", Password: "password1", Country: "us"},
		{Username: "user2-test", Password: "password2", Country: "eu"},
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
	require.NoError(t, batchErr)
}

func TestUpdateUser(t *testing.T) {
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-update-user")

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
		require.NoError(t, err)

		u, err := GetUserByID(ctx, pdb.Query, test.userID)
		require.NoError(t, err)

		assert.Equal(t, test.wantUsername, u.Username)
		assert.Equal(t, test.wantBio, u.Bio)
		assert.Equal(t, test.wantCountry, u.Country)
	}
}

func TestSelectOrInsertGoogleUser(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-insert-google-user")

	testAccountID := "test-account-id"

	inst := GoogleUserInst{Username: "username", Country: "us", JoinedOn: TestTimeNow}

	// when
	u1, err := SelectOrInsertGoogleUser(ctx, pdb.Query, testAccountID, inst)
	require.NoError(t, err)

	u2, err := SelectOrInsertGoogleUser(ctx, pdb.Query, testAccountID, inst)
	require.NoError(t, err)

	dbU1, err := GetUserByID(ctx, pdb.Query, u1.ID)
	require.NoError(t, err)

	// then
	verifiedUser := VerifiedUser{Username: "username", Country: "us"}
	assertutil.AssertEqualIgnoring(t, verifiedUser, u1, testVerifiedUserCmptOpts)
	assertutil.AssertEqualIgnoring(t, verifiedUser, u2, testVerifiedUserCmptOpts)

	wantU1 := UserEntity{
		Username: "username",
		Country:  "us",
		JoinedOn: TestTimeNow.Local(),
	}
	assertutil.AssertEqualIgnoring(t, wantU1, dbU1, testUserCmptOpts)
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "update-password")

	// when
	err := UpdateUserPassword(ctx, pdb.Query, TestUserEntities[0].ID, "password-new")
	require.NoError(t, err)

	u1, err := GetUserByID(ctx, pdb.Query, TestUserEntities[0].ID)
	require.NoError(t, err)
	v1, err := verifyUser(ctx, pdb.Query, TestUserEntities[0].Username, "password-new")
	require.NoError(t, err)

	// then
	assert.Equal(t, u1.ID, v1.ID)
}

func TestGetUserElos(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "get-user-elos")

	// when
	stats, err := GetUserStats(ctx, pdb.Query, 1)
	require.NoError(t, err)

	// then
	assertutil.AssertEqualIgnoring(t, TestUserStats[0], stats, cmpopts.IgnoreFields(ModeStatsEntity{}, "Rank"))
}
