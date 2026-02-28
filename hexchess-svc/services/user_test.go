package svc

import (
	"context"
	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserCmptOpts = cmpopts.IgnoreFields(UserEntity{}, "ID")
var testVerifiedUserCmptOpts = cmpopts.IgnoreFields(VerifiedUser{}, "ID")

func TestInsertThenVerify(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	user1 := "user1-testing"

	// when
	u1, err := services.InsertUser(ctx, UserInst{Username: user1, Password: "password1", Country: "us", JoinedOn: itest.TimeNow})
	require.NoError(t, err)

	v1, err := verifyUser(ctx, services.Query(), user1, "password1")
	require.NoError(t, err)

	dbU1, err := services.GetUserByID(ctx, v1.ID)
	require.NoError(t, err)

	var attemptsErrs []error
	for range LoginAttemptsDivisor {
		_, err := verifyUser(ctx, services.Query(), user1, "wrong-password")
		attemptsErrs = append(attemptsErrs, err)
	}
	_, errTooMany := verifyUser(ctx, services.Query(), user1, "wrong-password")

	// then
	var wantAttemptErrs []error
	for range LoginAttemptsDivisor {
		wantAttemptErrs = append(wantAttemptErrs, ErrUserNotFound)
	}
	assert.Equal(t, wantAttemptErrs, attemptsErrs)
	assert.Equal(t, ErrTooManyLoginAttempts, errTooMany)

	assert.Equal(t, u1.ID, v1.ID)
	wantU1 := UserEntity{
		Username: "user1-testing",
		Country:  "us",
		JoinedOn: itest.TimeNow.Local(),
	}
	testutil.Equal(t, wantU1, dbU1, testUserCmptOpts)
}

func TestBatchInsertThenGet(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	insts := []UserInst{
		{Username: "user1-testing", Password: "password1", Country: "us"},
		{Username: "user2-testing", Password: "password2", Country: "eu"},
	}
	users, batchErr := services.BatchInsertUsers(ctx, insts)

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
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

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
		_, err := services.UpdateUser(ctx, test.userID, test.udpt)
		require.NoError(t, err)

		u, err := services.GetUserByID(ctx, test.userID)
		require.NoError(t, err)

		assert.Equal(t, test.wantUsername, u.Username)
		assert.Equal(t, test.wantBio, u.Bio)
		assert.Equal(t, test.wantCountry, u.Country)
	}
}

func TestSelectOrInsertGoogleUser(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testAccountID := "testing-account-id"

	inst := GoogleUserInst{Username: "username", Country: "us", JoinedOn: itest.TimeNow}

	// when
	u1, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	u2, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	dbU1, err := services.GetUserByID(ctx, u1.ID)
	require.NoError(t, err)

	// then
	verifiedUser := VerifiedUser{Username: "username", Country: "us"}
	testutil.Equal(t, verifiedUser, u1, testVerifiedUserCmptOpts)
	testutil.Equal(t, verifiedUser, u2, testVerifiedUserCmptOpts)

	wantU1 := UserEntity{
		Username: "username",
		Country:  "us",
		JoinedOn: itest.TimeNow.Local(),
	}
	testutil.Equal(t, wantU1, dbU1, testUserCmptOpts)
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	err := services.UpdateUserPassword(ctx, TestUserEntities[0].ID, "password-new")
	require.NoError(t, err)

	u1, err := services.GetUserByID(ctx, TestUserEntities[0].ID)
	require.NoError(t, err)
	v1, err := verifyUser(ctx, services.Query(), TestUserEntities[0].Username, "password-new")
	require.NoError(t, err)

	// then
	assert.Equal(t, u1.ID, v1.ID)
}

func TestGetUserElos(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	stats, err := services.GetUserStats(ctx, 1)
	require.NoError(t, err)

	// then
	testutil.Equal(t, TestUserStats[0], stats, cmpopts.IgnoreFields(ModeStatsEntity{}, "Rank"))
}
