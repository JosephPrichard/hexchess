package data

import (
	"context"
	"fmt"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"hexchess-svc/db"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

const TestDbUser = "postgres"
const TestDbName = "hexchess"
const TestDbPass = "password123"
const TestDbPort = 9876

// locks allow these tests to be run concurrently
var muPostgres sync.Mutex
var postgres *embeddedpostgres.EmbeddedPostgres
var muRedis sync.Mutex
var redisCont *tcredis.RedisContainer

func TestMain(m *testing.M) {
	var exitCode int
	func() {
		defer teardownTestInfra()
		exitCode = m.Run()
	}()
	os.Exit(exitCode)
}

func teardownTestInfra() {
	if postgres != nil {
		if err := postgres.Stop(); err != nil {
			log.Fatalf("failed to stop test db with err: %v", err)
		}
		log.Printf("stopped test postgres db")
	}
	if redisCont != nil {
		if err := testcontainers.TerminateContainer(redisCont); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}
		log.Printf("stopped test redis container")
	}
}

type TestLogger interface {
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

func initRedis(t TestLogger) string {
	muRedis.Lock()
	defer muRedis.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	if redisCont == nil {
		start := time.Now()
		t.Logf("starting up the redis container")
		cont, err := tcredis.Run(ctx, "redis:6-alpine", testcontainers.WithExposedPorts("6379"))
		if err != nil {
			t.Fatalf("failed to start container: %s", err)
		}
		redisCont = cont
		t.Logf("finished starting up the redis container in %v", time.Now().Sub(start))
	}

	host, err := redisCont.Container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %s", err)
	}
	resp, err := redisCont.Container.Inspect(ctx)
	if err != nil {
		t.Fatalf("failed to get port: %s", err)
	}
	port := resp.NetworkSettings.Ports["6379/tcp"][0].HostPort
	return host + ":" + port
}

func beforeRedisTests(t TestLogger) (*redis.Pool, func()) {
	pool, _, closer := beforeRedisTestsWithAddr(t)
	return pool, closer
}

func beforeRedisTestsWithAddr(t TestLogger) (*redis.Pool, string, func()) {
	addr := initRedis(t)
	t.Logf("connecting to redis on addr: %s", addr)

	rdb := MakeRdbPool(addr)
	closer := func() { rdb.Close() }

	return rdb, addr, closer
}

func initEmbeddedPostgres(t TestLogger, pgDB DB) {
	muPostgres.Lock()
	defer muPostgres.Unlock()

	start := time.Now()

	// create the actual test postgres instance
	config := embeddedpostgres.DefaultConfig().
		Username(TestDbUser).
		Password(TestDbPass).
		Database(TestDbName).
		Version(embeddedpostgres.V15).
		RuntimePath("/tmp").
		Port(uint32(TestDbPort)).
		StartTimeout(30 * time.Second)
	t.Logf("starting the embedded test database with config.go: %v", config)

	postgres = embeddedpostgres.NewDatabase(config)
	if err := postgres.Start(); err != nil {
		t.Fatalf("failed to start embedded datanase: %v", err)
	}

	// initialize the schema and test data for the test postgres instance
	if _, err := pgDB.pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	if _, err := pgDB.pool.Exec(context.Background(), db.CreateSchema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	createTestUsers(t, pgDB, TestUsers...)
	createTestReplays(t, pgDB, TestReplays...)

	t.Logf("finished setting up the embedded test database in %v", time.Now().Sub(start))
}

func beforeDbTests(t TestLogger) (DB, func()) {
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%d", TestDbUser, TestDbName, TestDbPass, TestDbPort))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	closer := func() { pool.Close() }
	pgDB := MakeDbClient(db.New(pool), pool)

	if postgres == nil {
		initEmbeddedPostgres(t, pgDB)
	}

	return pgDB, closer
}
