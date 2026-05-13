package svc

import (
	"hexchess-svc/model"

	"testing"

	"hexchess-svc/internal/testutil"
	"hexchess-svc/itest"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserCmptOpts = cmpopts.IgnoreFields(model.User{}, "ID")
var testVerifiedUserCmptOpts = cmpopts.IgnoreFields(VerifiedUser{}, "ID")

func TestInsertThenVerify(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := t.Context()

	user1 := "user1-testing"

	users1, err := services.InsertUser(ctx, UserInst{Username: user1, Password: "password1", Country: "us", JoinedOn: itest.TimeNow})
	require.NoError(t, err)

	verifyUser1, err := services.VerifyUser(ctx, user1, "password1")
	require.NoError(t, err)

	dbUser1, err := services.GetUserByID(ctx, verifyUser1.ID)
	require.NoError(t, err)

	var attemptsErrs []error
	for range LoginAttemptsDivisor {
		_, err := services.VerifyUser(ctx, user1, "wrong-password")
		attemptsErrs = append(attemptsErrs, err)
	}
	_, errTooMany := services.VerifyUser(ctx, user1, "wrong-password")

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

			services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
			defer services.Close()

			ctx := t.Context()

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

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := t.Context()

	testAccountID := "testing-account-existingID"

	inst := GoogleUserInst{Username: "username", Country: "us", JoinedOn: itest.TimeNow}

	user1, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	user2, err := services.SelectOrInsertGoogleUser(ctx, testAccountID, inst)
	require.NoError(t, err)

	dbUser1, err := services.GetUserByID(ctx, user1.ID)
	require.NoError(t, err)

	verifiedUser := VerifiedUser{Username: "username", Country: "us"}
	testutil.Equal(t, verifiedUser, user1, testVerifiedUserCmptOpts)
	testutil.Equal(t, verifiedUser, user2, testVerifiedUserCmptOpts)

	wantDbUser1 := model.User{
		Username: "username",
		Country:  "us",
		JoinedOn: itest.TimeNow.Local(),
	}
	testutil.Equal(t, wantDbUser1, dbUser1, testUserCmptOpts)
}

func TestUpdatePasswordThenVerify(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := t.Context()

	err := services.UpdateUserPassword(ctx, itest.TestUser[0].ID, "password-new")
	require.NoError(t, err)

	u1, err := services.GetUserByID(ctx, itest.TestUser[0].ID)
	require.NoError(t, err)
	v1, err := services.VerifyUser(ctx, itest.TestUser[0].Username, "password-new")
	require.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func TestGetUserElos(t *testing.T) {
	t.Parallel()

	services, _ := setupServicesTest(t, serviceMocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := t.Context()

	stats, err := services.GetUserStats(ctx, 1)
	require.NoError(t, err)

	testutil.Equal(t, itest.TestUserStats[0], stats, cmpopts.IgnoreFields(model.ModeStats{}, "Rank"))
}
