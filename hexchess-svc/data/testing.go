package data

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"hexchess-svc/db"
	"hexchess-svc/logs"
	"log"
	"sync"
	"time"
)

const TestDbUser = "postgres"
const TestDbName = "postgres"
const TestDbPass = "postgres"

var muPostgres sync.Mutex
var muRedis sync.Mutex
var postgresCont testcontainers.Container
var redisCont *tcredis.RedisContainer

func TeardownTestInfra() {
	log.Print("tearing down test infra")
	if postgresCont != nil {
		if err := testcontainers.TerminateContainer(postgresCont); err != nil {
			log.Fatalf("failed to stop test db with err: %v", err)
		}
		log.Print("stopped test postgres db")
	}
	if redisCont != nil {
		if err := testcontainers.TerminateContainer(redisCont); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}
		log.Print("stopped test redis container")
	}
}

type TestLogger interface {
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

func BeforeRedisTests(t TestLogger) (Rdb, func()) {
	rdb, _, closer := BeforeRedisTestsWithAddr(t)
	return rdb, closer
}

func BeforeRedisTestsWithAddr(t TestLogger) (Rdb, string, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	muRedis.Lock()
	defer muRedis.Unlock()

	if redisCont == nil {
		start := time.Now()
		t.Logf("starting up the redis container")
		cont, err := tcredis.Run(ctx, "redis:6-alpine", testcontainers.WithExposedPorts("6379"))
		if err != nil {
			t.Fatalf("failed to start cont: %s", err)
		}
		redisCont = cont
		t.Logf("finished starting up the redis cont in %v", time.Now().Sub(start))
	}

	host, err := redisCont.Container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis cont host: %s", err)
	}
	port, err := redisCont.Container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("failed to get redis port: %s", err)
	}
	addr := host + ":" + port.Port()

	t.Logf("connecting to redis on addr: %s", addr)

	rdb := MakeRdb(addr)
	// make unique ZSET names so any test that uses this rdb instance is isolated
	rdb.LeaderboardZSet += uuid.NewString()
	rdb.ActiveUsersZSet += uuid.NewString()
	rdb.GamesZSet += uuid.NewString()

	closer := func() {
		t.Logf("closing redis pool")
		rdb.Close()
	}
	return rdb, addr, closer
}

func BeforeDbTests(t TestLogger) (PgDB, func()) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	muPostgres.Lock()
	defer muPostgres.Unlock()

	shouldSeed := false
	if postgresCont == nil {
		req := testcontainers.ContainerRequest{
			Image:        "postgres:16",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     TestDbUser,
				"POSTGRES_PASSWORD": TestDbPass,
				"POSTGRES_DB":       TestDbName,
			},
			WaitingFor: wait.ForListeningPort("5432/tcp"),
		}
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			t.Fatalf("failed to start postgres cont: %s", err)
		}
		postgresCont = cont
		shouldSeed = true
	}

	host, err := postgresCont.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get postgres cont host: %s", err)
	}
	port, err := postgresCont.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("failed to get postgres port: %s", err)
	}

	//pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%s", "postgres", "hexachess2", "hurricane123", "5432"))
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", TestDbUser, TestDbPass, host, port.Port(), TestDbName))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	q := db.New(pool)

	if shouldSeed {
		// initialize the schema and test data for the test postgres instance, but only after the container is created
		if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
			t.Fatalf("failed to create schema: %v", err)
		}
		if _, err := pool.Exec(ctx, db.CreateSchema); err != nil {
			t.Fatalf("failed to create schema: %v", err)
		}

		createTestUsers(t, q, TestUsersInsts...)
		createTestReplays(t, q, TestReplayInsts...)

		t.Logf("finished setting up postgres test cont in %v", time.Now().Sub(start))
	}

	testTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to open testing tx %v", err)
	}

	closer := func() {
		t.Logf("shutting down test txn and pool")
		if err := testTx.Rollback(context.Background()); err != nil {
			t.Fatalf("failed to rollback test txn: %v", err)
		}
		pool.Close()
	}

	q = q.WithTx(testTx)
	return MakeFakeDbClient(q), closer
}

func BeforeStoresTests(t TestLogger) (Stores, func()) {
	pgDB, dbCloser := BeforeDbTests(t)
	rdb, rdbCloser := BeforeRedisTests(t)
	closer := func() {
		dbCloser()
		rdbCloser()
	}
	return Stores{PgDB: pgDB, Rdb: rdb}, closer
}

var TestUsersInsts = []UserInst{
	{Username: "user1", Password: "password1", Country: "us", Elo: 1000},
	{Username: "user2", Password: "password2", Country: "us", Elo: 1000, Wins: 1},
	{Username: "user3", Password: "password3", Country: "us", Elo: 900, Wins: 1, Losses: 8},
	{Username: "user4", Password: "password4", Country: "us", Elo: 2000, Wins: 50, Losses: 20},
	{Username: "user5", Password: "password5", Country: "us", Elo: 1500, Wins: 40, Losses: 35},
}

var TestUserEntities = []UserEntity{
	{
		ID:         1,
		Username:   "user1",
		Country:    "us",
		Elo:        1000,
		HighestElo: 1000,
		Wins:       0,
		Losses:     0,
		Rank:       1,
		Bio:        "",
		Total:      0,
		Winrate:    0,
	},
}

func createTestUser(t TestLogger, q *db.Queries, inst UserInst) UserEntity {
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-testing-user")
	u, err := InsertUser(ctx, q, inst)
	if err != nil {
		t.Fatalf("failed to insert testing user: %v", err)
	}
	return u
}

func createTestUsers(t TestLogger, q *db.Queries, insts ...UserInst) []UserEntity {
	var users []UserEntity
	for _, inst := range insts {
		users = append(users, createTestUser(t, q, inst))
	}
	return users
}

var TestReplayInsts = []ReplayInst{
	{1, 2, int32(WhiteWin), int32(Checkmate), 30, -30, "[]"},
	{2, 3, int32(BlackWin), int32(Checkmate), 30, -30, "{}"},
	{3, 1, int32(Draw), int32(Checkmate), 30, -30, "{}"},
}

var TestReplayEntities = []ReplayEntity{
	{
		ID:           3,
		WhiteID:      3,
		BlackID:      1,
		WhiteName:    "user3",
		BlackName:    "user1",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       Draw,
		Cause:        Checkmate,
		WinElo:       30,
		LoseElo:      -30,
		WhiteElo:     900,
		BlackElo:     1000,
		WhiteEloDiff: 0,
		BlackEloDiff: 0,
	},
	{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Result:       WhiteWin,
		Cause:        Checkmate,
		WinElo:       30,
		LoseElo:      -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
	},
}

func createTestReplays(t TestLogger, q *db.Queries, insts ...ReplayInst) {
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-testing-replays")
	for _, inst := range insts {
		_, err := InsertReplay(ctx, q, inst)
		if err != nil {
			t.Fatalf("failed to insert testing replay: %v", err)
		}
	}
}
