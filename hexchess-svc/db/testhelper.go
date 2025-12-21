package db

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"hexchess-svc/util"
	"log"
	"sync"
	"time"
)

const TestDbUser = "postgres"
const TestDbName = "postgres"
const TestDbPass = "postgres"

const RedisContTag = "redis:8.4.0"
const PostgresContTag = "postgres:17"

var muPostgres sync.Mutex
var muRedis sync.Mutex
var postgresCont testcontainers.Container
var redisCont *tcredis.RedisContainer

func TeardownTestInfra() {
	log.Print("tearing down test infra")
	if postgresCont != nil {
		if err := testcontainers.TerminateContainer(postgresCont); err != nil {
			util.LogFatalErr("stop test db with err", err)
		}
		log.Print("stopped test postgres db")
	}
	if redisCont != nil {
		if err := testcontainers.TerminateContainer(redisCont); err != nil {
			util.LogFatalErr("terminate container", err)
		}
		log.Print("stopped test redis container")
	}
}

type TestLogger interface {
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

func startRedisContainer(t TestLogger) *tcredis.RedisContainer {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	t.Logf("starting redis container")
	cont, err := tcredis.Run(ctx, RedisContTag, testcontainers.WithExposedPorts("6379"))
	if err != nil {
		t.Fatalf("start redis container: %s", err)
	}
	t.Logf("finished starting redis container in %v", time.Since(start))
	return cont
}

func getRedisAddr(t TestLogger, cont *tcredis.RedisContainer) string {
	ctx := context.Background()

	host, err := cont.Container.Host(ctx)
	if err != nil {
		t.Fatalf("get redis host: %s", err)
	}
	port, err := cont.Container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("get redis port: %s", err)
	}

	addr := fmt.Sprintf("%s:%s", host, port.Port())
	t.Logf("redis addr: %s", addr)
	return addr
}

func BeforeRedisTest(t TestLogger) *Redis {
	muRedis.Lock()
	defer muRedis.Unlock()

	if redisCont == nil {
		redisCont = startRedisContainer(t)
	}

	addr := getRedisAddr(t, redisCont)
	return MakeRdb(
		RedisAddrs{addr, addr},
		RedisNames{
			LeaderboardZSet: LeaderboardZSet + uuid.NewString(),
			GamesZSet:       GamesZSet + uuid.NewString(),
			ActiveUsersZSet: ActiveUsersZSet + uuid.NewString(),
			GamesChan:       GamesChan + uuid.NewString(),
			UsersChan:       UsersChan + uuid.NewString(),
			GamesCountChan:  GamesCountChan + uuid.NewString(),
			ActiveCountChan: ActiveCountChan + uuid.NewString(),
		},
	)
}

func startPostgresContainer(t TestLogger) testcontainers.Container {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        PostgresContTag,
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
		t.Fatalf("start postgres container: %s", err)
	}

	return cont
}

func createPgPool(t TestLogger, cont testcontainers.Container) *pgxpool.Pool {
	ctx := context.Background()
	host, err := cont.Host(ctx)
	if err != nil {
		t.Fatalf("get postgres container host: %s", err)
	}
	port, err := cont.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("get postgres container port: %s", err)
	}
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", TestDbUser, TestDbPass, host, port.Port(), TestDbName))
	if err != nil {
		t.Fatalf("create pgx pool: %v", err)
	}
	return pool
}

func seedPostgres(t TestLogger, pool *pgxpool.Pool, insertTestData func(TestLogger, *pgxpool.Pool)) {
	ctx := context.Background()
	var version string
	if err := pool.QueryRow(ctx, "SELECT version();").Scan(&version); err != nil {
		t.Fatalf("get postgres version: %v", err)
	}
	t.Logf("seeding postgres version: %s", version)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if _, err := pool.Exec(ctx, CreateSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	insertTestData(t, pool)
}

func beginTestTx(t TestLogger, pool *pgxpool.Pool) (pgx.Tx, func()) {
	ctx := context.Background()
	testTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("open test tx: %v", err)
	}
	closer := func() {
		if err := testTx.Rollback(context.Background()); err != nil {
			t.Fatalf("rollback test tx: %v", err)
		}
		t.Logf("rolled back test tx")
		pool.Close()
	}
	return testTx, closer
}

func BeforePostgresTest(t TestLogger, useTestTx bool, insertTestData func(TestLogger, *pgxpool.Pool)) (*PostgreSQL, func()) {
	muPostgres.Lock()
	defer muPostgres.Unlock()

	shouldSeed := false
	if postgresCont == nil {
		postgresCont = startPostgresContainer(t)
		shouldSeed = true
	}

	pool := createPgPool(t, postgresCont)
	if shouldSeed {
		seedPostgres(t, pool, insertTestData)
	}

	if useTestTx {
		txn, closer := beginTestTx(t, pool)
		return MakeTestTxnPostgres(txn), closer
	} else {
		return MakePostgres(New(pool), pool), func() { pool.Close() }
	}
}

func BeforeDbTest(t TestLogger, useTx bool, insertTestData func(TestLogger, *pgxpool.Pool)) (Databases, func()) {
	pdb, pdbCloser := BeforePostgresTest(t, useTx, insertTestData)
	rdb := BeforeRedisTest(t)
	return Databases{Pdb: pdb, Rdb: rdb}, func() { pdbCloser(); rdb.Close() }
}
