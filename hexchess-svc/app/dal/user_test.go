package dal

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

var TestUsers = []UserInst{
	{Username: "user1", Password: "password1", Country: "us", Elo: 1000},
	{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1},
	{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8},
	{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20},
	{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35},
}

func createTestUser(t *testing.T, pgDB DB, inst UserInst) UserEntity {
	ctx := context.WithValue(context.Background(), util.TraceKey, "create-test-user")
	u, err := InsertUser(ctx, pgDB.Q, inst)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	return u
}

func createTestUsers(t *testing.T, pgDB DB, insts ...UserInst) []UserEntity {
	var users []UserEntity
	for _, inst := range insts {
		users = append(users, createTestUser(t, pgDB, inst))
	}
	return users
}

func TestInsertThenVerify(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-insert-then-verify")

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

//
//func TestBatchInsertThenGet(t *testing.T) {
//	pgDB, closer := beforeDbTests(t)
//	defer closer()
//
//	ctx := context.WithValue(context.Background(), util.TraceKey, "test-batch-insert-then-get")
//
//	users := []UserInst{
//		{Username: "user1", Password: "password1", Country: "us", Elo: 1005, Wins: 10, Losses: 9},
//		{Username: "user2", Password: "password2", Country: "eu", Elo: 1035, Wins: 12, Losses: 9},
//	}
//	assert.NoError(t, BatchInsertUsers(ctx, pgDB.Q, users))
//
//	u1, err := GetUserById(ctx, pgDB.Q, 1)
//	assert.NoError(t, err)
//	u2, err := GetUserById(ctx, pgDB.Q, 2)
//	assert.NoError(t, err)
//
//	v1, err := VerifyUser(ctx, pgDB.Q, "user1", "password1")
//	assert.NoError(t, err)
//	v2, err := VerifyUser(ctx, pgDB.Q, "user2", "password2")
//	assert.NoError(t, err)
//
//	assert.Equal(t, u1.ID, v1.ID)
//	assert.Equal(t, u2.ID, v2.ID)
//}

//func testUpdateStatsRollback(t *testing.T, pgDB DB) {
//	ctx := context.Background()
//
//	createTestUsers(t, pgDB.Q)
//
//	userDao.MockProbWins = func(_, _ float64) (float64, error) {
//		panic("mocked panic")
//	}
//
//	func () {
//		defer func() {
//			if err := recover(); err != nil {
//				t.Logf("panic recovered: %s", err)
//			}
//		}()
//		_, _ = UpdateGameResultTx(ctx, pgDB, 1, 3)
//	}()
//
//	u1, err := GetUserById(ctx, pgDB.pgDB.Q, 1)
//	assert.NoError(t, err)
//	assert.Equal(t, float64(1000), u1.Elo)
//}

func TestUpdateUser(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	tests := []struct {
		inst        UserInst
		udpt        UpdateUserParams
		expUsername string
		expBio      string
		expCountry  string
	}{
		{
			inst:        UserInst{Username: "user1" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000},
			udpt:        UpdateUserParams{Username: "user1-changed", Bio: "Testing123"},
			expUsername: "user1-changed",
			expBio:      "Testing123",
			expCountry:  "us",
		},
		{
			inst:        UserInst{Username: "user2" + uuid.NewString(), Password: "password2", Country: "us", Elo: 1000},
			udpt:        UpdateUserParams{Username: "user2-changed", Country: "eu"},
			expUsername: "user2-changed",
			expBio:      "",
			expCountry:  "eu",
		},
	}

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-update-user")

	for _, test := range tests {
		testUser := createTestUser(t, pgDB, test.inst)

		assert.NoError(t, UpdateUser(ctx, pgDB.Q, testUser.ID, test.udpt))

		u, err := GetUserById(ctx, pgDB.Q, testUser.ID)
		assert.NoError(t, err)

		assert.Equal(t, test.expUsername, u.Username)
		assert.Equal(t, test.expBio, u.Bio)
		assert.Equal(t, test.expCountry, u.Country)
	}
}

func TestUpdatePassword(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	testUser := createTestUser(t, pgDB, UserInst{Username: "user1-" + uuid.NewString(), Password: "password1", Country: "us", Elo: 1000})

	ctx := context.WithValue(context.Background(), util.TraceKey, "update-password")

	assert.NoError(t, UpdateUserPassword(ctx, pgDB.Q, testUser.ID, "password-new"))

	u1, err := GetUserById(ctx, pgDB.Q, testUser.ID)
	assert.NoError(t, err)
	v1, err := VerifyUser(ctx, pgDB.Q, testUser.Username, "password-new")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func TestSearchByName(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	createTestUsers(t, pgDB, UserInst{Username: "johnny", Password: "password6"}, UserInst{Username: "john", Password: "password7"})

	ctx := context.WithValue(context.Background(), util.TraceKey, "search-by-name")

	list, err := SearchUsersByName(ctx, pgDB.Q, "john", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}
