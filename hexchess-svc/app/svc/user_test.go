package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"math"
	"testing"
)

func createTestUsers(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "create-test-data")
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user1", Password: "password1", Country: "us", Elo: 1000}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35}))
}

func TestInsertThenVerify(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-insert-then-verify")

	u1, err := InsertUserRet(ctx, pgDB.Q, UserInst{Username: "user1", Password: "password1"})
	assert.NoError(t, err)
	u2, err := InsertUserRet(ctx, pgDB.Q, UserInst{Username: "user2", Password: "password2"})
	assert.NoError(t, err)
	u3, err := InsertUserRet(ctx, pgDB.Q, UserInst{Username: "user3", Password: "password3"})
	assert.NoError(t, err)

	v1, err := VerifyUser(ctx, pgDB.Q, "user1", "password1")
	assert.NoError(t, err)
	v2, err := VerifyUser(ctx, pgDB.Q, "user2", "password2")
	assert.NoError(t, err)
	_, err1 := VerifyUser(ctx, pgDB.Q, "user2", "wrong-password")
	v4, err := VerifyUser(ctx, pgDB.Q, "user3", "password3")
	assert.NoError(t, err)
	_, err2 := VerifyUser(ctx, pgDB.Q, "user1", "password3")

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
	assert.Error(t, err1)
	assert.Equal(t, u3.ID, v4.ID)
	assert.Error(t, err2)
}

func TestBatchInsertThenGet(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-batch-insert-then-get")

	users := []UserInst{
		{Username: "user1", Password: "password1", Country: "us", Elo: 1005, Wins: 10, Losses: 9},
		{Username: "user2", Password: "password2", Country: "eu", Elo: 1035, Wins: 12, Losses: 9},
	}
	assert.NoError(t, BatchInsertUsers(ctx, pgDB.Q, users))

	u1, err := GetUserById(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, pgDB.Q, 2)
	assert.NoError(t, err)

	v1, err := VerifyUser(ctx, pgDB.Q, "user1", "password1")
	assert.NoError(t, err)
	v2, err := VerifyUser(ctx, pgDB.Q, "user2", "password2")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
}

func TestUpdateStats(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-update-stats")
	createTestUsers(t, pgDB.Q)

	cs, err := UpdateUserStatsTx(ctx, pgDB, 1, 2)
	assert.NoError(t, err)
	u1, err := GetUserById(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, pgDB.Q, 2)
	assert.NoError(t, err)

	// assert a value relatively close to the actual value
	cs.WinEloDiff = math.Round(cs.WinEloDiff)
	cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
	u1.Elo = math.Round(u1.Elo)
	u2.Elo = math.Round(u2.Elo)

	expectedChange := EloChangeSet{WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expectedChange, cs)

	assert.Equal(t, float64(1015), u1.Elo)
	assert.Equal(t, float64(985), u2.Elo)
}

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
//		_, _ = UpdateUserStatsTx(ctx, pgDB, 1, 3)
//	}()
//
//	u1, err := GetUserById(ctx, pgDB.pgDB.Q, 1)
//	assert.NoError(t, err)
//	assert.Equal(t, float64(1000), u1.Elo)
//}

func TestUpdateUser(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-update-user")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, UpdateUser(ctx, pgDB.Q, 1, "user1-changed", "Testing123", ""))
	assert.NoError(t, UpdateUser(ctx, pgDB.Q, 2, "user2-changed", "", "eu"))

	u1, err := GetUserById(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, pgDB.Q, 2)
	assert.NoError(t, err)

	assert.Equal(t, "user1-changed", u1.Username)
	assert.Equal(t, "Testing123", u1.Bio)
	assert.Equal(t, "us", u1.Country)

	assert.Equal(t, "user2-changed", u2.Username)
	assert.Equal(t, "", u2.Bio)
	assert.Equal(t, "eu", u2.Country)
}

func TestUpdatePassword(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "update-password")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, UpdateUserPassword(ctx, pgDB.Q, 1, "password-new"))

	u1, err := GetUserById(ctx, pgDB.Q, 1)
	assert.NoError(t, err)
	v1, err := VerifyUser(ctx, pgDB.Q, "user1", "password-new")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func TestSearchByName(t *testing.T) {
	pgDB, closer := initDbClient(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "search-by-name")
	createTestUsers(t, pgDB.Q)

	assert.NoError(t, InsertUser(ctx, pgDB.Q, UserInst{Username: "johnny", Password: "password6"}))
	assert.NoError(t, InsertUser(ctx, pgDB.Q, UserInst{Username: "john", Password: "password7"}))

	list, err := SearchUsersByName(ctx, pgDB.Q, "john", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}
