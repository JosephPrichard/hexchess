package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"math"
	"testing"
)

func TestUserQueries(t *testing.T) {
	closer := createEmbeddedDb(t)
	defer closer()

	pool := makeTestPgxPool(t)
	defer pool.Close()

	q := db.New(pool)
	qtx := MakeQueriesTx(q, pool)

	t.Run("TestInsertThenVerify", func(t *testing.T) {
		resetTestSchema(t, pool)
		testInsertThenVerify(t, q)
	})
	t.Run("TestBatchInsertThenGet", func(t *testing.T) {
		resetTestSchema(t, pool)
		testBatchInsertThenGet(t, q)
	})
	t.Run("TestUpdateStats", func(t *testing.T) {
		resetTestSchema(t, pool)
		testUpdateStats(t, qtx)
	})
	t.Run("TestUpdateUser", func(t *testing.T) {
		resetTestSchema(t, pool)
		testUpdateUser(t, q)
	})
	t.Run("TestUpdatePassword", func(t *testing.T) {
		resetTestSchema(t, pool)
		testUpdatePassword(t, q)
	})
	t.Run("TestSearchByName", func(t *testing.T) {
		resetTestSchema(t, pool)
		testSearchByName(t, q)
	})
}

func createTestUsers(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "create-test-data")
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user1", Password: "password1", Country: "us", Elo: 1000}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35}))
}

func testInsertThenVerify(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "test-insert-then-verify")

	u1, err := InsertUserRet(ctx, q, UserInst{Username: "user1", Password: "password1"})
	assert.NoError(t, err)
	u2, err := InsertUserRet(ctx, q, UserInst{Username: "user2", Password: "password2"})
	assert.NoError(t, err)
	u3, err := InsertUserRet(ctx, q, UserInst{Username: "user3", Password: "password3"})
	assert.NoError(t, err)

	v1, err := VerifyUser(ctx, q, "user1", "password1")
	assert.NoError(t, err)
	v2, err := VerifyUser(ctx, q, "user2", "password2")
	assert.NoError(t, err)
	_, err1 := VerifyUser(ctx, q, "user2", "wrong-password")
	v4, err := VerifyUser(ctx, q, "user3", "password3")
	assert.NoError(t, err)
	_, err2 := VerifyUser(ctx, q, "user1", "password3")

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
	assert.Error(t, err1)
	assert.Equal(t, u3.ID, v4.ID)
	assert.Error(t, err2)
}

func testBatchInsertThenGet(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "test-batch-insert-then-get")

	users := []UserInst{
		{Username: "user1", Password: "password1", Country: "us", Elo: 1005, Wins: 10, Losses: 9},
		{Username: "user2", Password: "password2", Country: "eu", Elo: 1035, Wins: 12, Losses: 9},
	}
	assert.NoError(t, BatchInsertUsers(ctx, q, users))

	u1, err := GetUserById(ctx, q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, q, 2)
	assert.NoError(t, err)

	v1, err := VerifyUser(ctx, q, "user1", "password1")
	assert.NoError(t, err)
	v2, err := VerifyUser(ctx, q, "user2", "password2")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
	assert.Equal(t, u2.ID, v2.ID)
}

func testUpdateStats(t *testing.T, qtx QueriesTx) {
	ctx := context.WithValue(context.Background(), TraceKey, "test-update-stats")

	createTestUsers(t, qtx.Q)

	cs, err := UpdateUserStatsTx(ctx, qtx, 1, 2)
	assert.NoError(t, err)
	u1, err := GetUserById(ctx, qtx.Q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, qtx.Q, 2)
	assert.NoError(t, err)

	cs.WinEloDiff = math.Round(cs.WinEloDiff)
	cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
	u1.Elo = math.Round(u1.Elo)
	u2.Elo = math.Round(u2.Elo)

	expectedChange := EloChangeSet{WinEloDiff: 15, LoseEloDiff: -15}
	assert.Equal(t, expectedChange, cs)

	assert.Equal(t, float64(1015), u1.Elo)
	assert.Equal(t, float64(985), u2.Elo)
}

//func testUpdateStatsRollback(t *testing.T, qtx QueriesTx) {
//	ctx := context.Background()
//
//	createTestUsers(t, qtx.Q)
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
//		_, _ = UpdateUserStatsTx(ctx, qtx, 1, 3)
//	}()
//
//	u1, err := GetUserById(ctx, qtx.Q, 1)
//	assert.NoError(t, err)
//	assert.Equal(t, float64(1000), u1.Elo)
//}

func testUpdateUser(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "test-update-user")

	createTestUsers(t, q)

	assert.NoError(t, UpdateUser(ctx, q, 1, "user1-changed", "Testing123", ""))
	assert.NoError(t, UpdateUser(ctx, q, 2, "user2-changed", "", "eu"))

	u1, err := GetUserById(ctx, q, 1)
	assert.NoError(t, err)
	u2, err := GetUserById(ctx, q, 2)
	assert.NoError(t, err)

	assert.Equal(t, "user1-changed", u1.Username)
	assert.Equal(t, "Testing123", u1.Bio)
	assert.Equal(t, "us", u1.Country)

	assert.Equal(t, "user2-changed", u2.Username)
	assert.Equal(t, "", u2.Bio)
	assert.Equal(t, "eu", u2.Country)
}

func testUpdatePassword(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "update-password")

	createTestUsers(t, q)

	assert.NoError(t, UpdateUserPassword(ctx, q, 1, "password-new"))

	u1, err := GetUserById(ctx, q, 1)
	assert.NoError(t, err)
	v1, err := VerifyUser(ctx, q, "user1", "password-new")
	assert.NoError(t, err)

	assert.Equal(t, u1.ID, v1.ID)
}

func testSearchByName(t *testing.T, q *db.Queries) {
	ctx := context.WithValue(context.Background(), TraceKey, "search-by-name")

	createTestUsers(t, q)

	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "johnny", Password: "password6"}))
	assert.NoError(t, InsertUser(ctx, q, UserInst{Username: "john", Password: "password7"}))

	list, err := SearchUsersByName(ctx, q, "john", 1, 20)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
}
