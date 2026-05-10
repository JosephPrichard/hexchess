package svc

import (
	"context"
	"hexchess-svc/model"

	"testing"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserCmptOpts = cmpopts.IgnoreFields(model.User{}, "ID")
var testVerifiedUserCmptOpts = cmpopts.IgnoreFields(VerifiedUser{}, "ID")

func TestInsertThenVerify(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	user1 := "user1-testing"

	users1, err := services.InsertUser(ctx, UserInst{Username: user1, Password: "password1", Country: "us", JoinedOn: itest.TimeNow})
	require.NoError(t, err)

	verifyUser1, err := services.VerifyUserTx(ctx, user1, "password1")
	require.NoError(t, err)

	dbUser1, err := services.GetUserByID(ctx, verifyUser1.ID)
	require.NoError(t, err)

	var attemptsErrs []error
	for range LoginAttemptsDivisor {
		_, err := services.VerifyUserTx(ctx, user1, "wrong-password")
		attemptsErrs = append(attemptsErrs, err)
	}
	_, errTooMany := services.VerifyUserTx(ctx, user1, "wrong-password")

	var wantAttemptErrs []error
	for range LoginAttemptsDivisor {
		wantAttemptErrs = append(wantAttemptErrs, ErrUserNotFound)
	}
	assert.Equal(t, wantAttemptErrs, attemptsErrs)
	assert.Equal(t, ErrTooManyLoginAttempts, errTooMany)

	assert.Equal(t, users1.ID, verifyUser1.ID)
	wantDBU1 := model.User{
		Username: "user1-testing",
		Country:  "us",
		JoinedOn: itest.TimeNow.Local(),
	}
	testutil.Equal(t, wantDBU1, dbUser1, testUserCmptOpts)
}

func TestBatchInsertThenGet(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	insts := []UserInst{
		{Username: "user1-testing", Password: "password1", Country: "us"},
		{Username: "user2-testing", Password: "password2", Country: "eu"},
	}
	users, batchErr := services.BatchInsertUsers(ctx, insts)

	wantUsers := []model.User{
		{Username: insts[0].Username, Country: "us"},
		{Username: insts[1].Username, Country: "eu"},
	}

	testutil.Equal(t, wantUsers, users, cmpopts.IgnoreFields(model.User{}, "ID", "JoinedOn"))
	require.NoError(t, batchErr)
}

func TestUpdateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userID       int64
		udpt         UpdtUserParams
		wantUsername string
		wantBio      string
		wantCountry  string
	}{
		{
			name:         "UpdateNameAndBio",
			userID:       1,
			udpt:         UpdtUserParams{Username: "user1-changed", Bio: "Testing123"},
			wantUsername: "user1-changed",
			wantBio:      "Testing123",
			wantCountry:  "us",
		},
		{
			name:         "UpdateNameAndCountry",
			userID:       2,
			udpt:         UpdtUserParams{Username: "user2-changed", Country: "eu"},
			wantUsername: "user2-changed",
			wantBio:      "",
			wantCountry:  "eu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			_, err := services.UpdateUser(ctx, tt.userID, tt.udpt)
			require.NoError(t, err)

			user, err := services.GetUserByID(ctx, tt.userID)
			require.NoError(t, err)

			assert.Equal(t, tt.wantUsername, user.Username)
			assert.Equal(t, tt.wantBio, user.Bio)
			assert.Equal(t, tt.wantCountry, user.Country)
		})
	}
}

func TestSelectOrInsertGoogleUser(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testAccountID := "testing-account-existingID"

	inst := GoogleUserInst{Username: "incomingUsername", Country: "us", JoinedOn: itest.TimeNow}

	user1, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	user2, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	dbUser1, err := services.GetUserByID(ctx, user1.ID)
	require.NoError(t, err)

	verifiedUser := VerifiedUser{Username: "incomingUsername", Country: "us"}
	testutil.Equal(t, verifiedUser, user1, testVerifiedUserCmptOpts)
	testutil.Equal(t, verifiedUser, user2, testVerifiedUserCmptOpts)

	wantDbUser1 := model.User{
		Username: "incomingUsername",
		Country:  "us",
		JoinedOn: itest.TimeNow.Local(),
	}
	testutil.Equal(t, wantDbUser1, dbUser1, testUserCmptOpts)
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	err := services.UpdateUserPassword(ctx, itest.TestUser[0].ID, "password-new")
	require.NoError(t, err)

	u1, err := services.GetUserByID(ctx, itest.TestUser[0].ID)
	require.NoError(t, err)
	v1, err := services.VerifyUserTx(ctx, itest.TestUser[0].Username, "password-new")
	require.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func TestGetUserElos(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	stats, err := services.GetUserStats(ctx, 1)
	require.NoError(t, err)

	testutil.Equal(t, itest.TestUserStats[0], stats, cmpopts.IgnoreFields(model.ModeStats{}, "Rank"))
}
