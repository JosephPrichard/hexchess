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
	"hexchess-svc/util"
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
			util.LogFatalErr("failed to stop test db with err", err)
		}
		log.Print("stopped test postgres db")
	}
	if redisCont != nil {
		if err := testcontainers.TerminateContainer(redisCont); err != nil {
			util.LogFatalErr("failed to terminate container", err)
		}
		log.Print("stopped test redis container")
	}
}

type TestLogger interface {
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

func BeforeRedisTests(t TestLogger) *Redis {
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

	rdb := MakeRdb(addr, addr)

	// make unique ZSET names so any test that uses this rdb instance is isolated
	rdb.LeaderboardZSet += "_" + uuid.NewString()
	rdb.ActiveUsersZSet += "_" + uuid.NewString()
	rdb.GamesZSet += "_" + uuid.NewString()

	return rdb
}

func BeforeDbTests(t TestLogger, useTestTx bool) (*Postgres, func()) {
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
		CreateTestData(t, q)
		t.Logf("finished setting up postgres test cont in %v", time.Now().Sub(start))
	}

	if useTestTx {
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
		return MakeFakePostgres(q), closer
	} else {
		closer := func() {
			t.Logf("shutting down pool")
			pool.Close()
		}
		return MakePostgres(q, pool), closer
	}
}

func BeforeDatabasesTests(t TestLogger, useTx bool) (Databases, func()) {
	postgres, dbCloser := BeforeDbTests(t, useTx)
	rdb := BeforeRedisTests(t)
	closer := func() {
		dbCloser()
		rdb.Close()
	}
	return Databases{Pdb: postgres, Rdb: rdb}, closer
}
