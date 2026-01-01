package db

import (
	"context"
	"fmt"
	"hexchess-svc/pkg/logutil"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const TestDbUser = "postgres"
const TestDbName = "postgres"
const TestDbPass = "postgres"

const RedisContTag = "redis:8.4.0"
const PostgresContTag = "postgres:17"

var muPostgres sync.Mutex
var muRedis sync.Mutex
var postgresCont testcontainers.Container
var redisCont *redis.RedisContainer

func TeardownTestInfra() {
	log.Print("tearing down test infra")
	if postgresCont != nil {
		if err := testcontainers.TerminateContainer(postgresCont); err != nil {
			logutil.FatalErr("stop test db with err", err)
		}
		log.Print("stopped test postgres db")
	}
	if redisCont != nil {
		if err := testcontainers.TerminateContainer(redisCont); err != nil {
			logutil.FatalErr("terminate container", err)
		}
		log.Print("stopped test redis container")
	}
}

func BeforeRedisTest(t logutil.TestLogger) *Redis {
	muRedis.Lock()
	defer muRedis.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if redisCont == nil {
		start := time.Now()
		t.Logf("starting redis container")
		cont, err := redis.Run(ctx, RedisContTag, testcontainers.WithExposedPorts("6379"))
		if err != nil {
			t.Fatalf("start redis container: %s", err)
		}

		redisCont = cont
		t.Logf("finished starting redis container in %v", time.Since(start))
	}

	host, err := redisCont.Container.Host(ctx)
	if err != nil {
		t.Fatalf("get redis host: %s", err)
	}
	port, err := redisCont.Container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("get redis port: %s", err)
	}

	addr := fmt.Sprintf("%s:%s", host, port.Port())
	t.Logf("redis addr: %s", addr)

	return MakeRdb(
		RedisAddrs{addr, addr},
		RedisNames{
			LeaderboardZSet: LeaderboardZSet + "_" + uuid.NewString(),
			GamesZSet:       GamesZSet + "_" + uuid.NewString(),
			ActiveUsersZSet: ActiveUsersZSet + "_" + uuid.NewString(),
			GamesChan:       GamesChan + "_" + uuid.NewString(),
			UsersChan:       UsersChan + "_" + uuid.NewString(),
			GamesCountChan:  GamesCountChan + "_" + uuid.NewString(),
			ActiveCountChan: ActiveCountChan + "_" + uuid.NewString(),
		},
	)
}

func BeforePostgresTest(t logutil.TestLogger, useTestTx bool) (*Postgres, func()) {
	muPostgres.Lock()
	defer muPostgres.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shouldSeed := false
	if postgresCont == nil {
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
			Started:          true,
			ContainerRequest: req,
		})
		if err != nil {
			t.Fatalf("failed to start postgres container: %s", err)
		}
		postgresCont = cont
		shouldSeed = true
	}

	host, err := postgresCont.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get postgres container host: %s", err)
	}
	port, err := postgresCont.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("failed to get postgres container port: %s", err)
	}
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", TestDbUser, TestDbPass, host, port.Port(), TestDbName))
	if err != nil {
		t.Fatalf("failed to create pgx conn: %v", err)
	}

	if shouldSeed {
		var version string
		if err := pool.QueryRow(ctx, "SELECT version();").Scan(&version); err != nil {
			t.Fatalf("failed to get postgres version: %v", err)
		}
		t.Logf("seeding postgres version: %s", version)
		if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
			t.Fatalf("failed to reset schema: %v", err)
		}
		if _, err := pool.Exec(ctx, CreateSchema); err != nil {
			t.Fatalf("failed to create schema: %v", err)
		}
		insertTestData(t, pool)
	}

	var pdb *Postgres
	if useTestTx {
		testTx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to open test tx: %v", err)
		}
		pdb = MakeTestTxnPostgres(testTx)
	} else {
		pdb = MakePostgres(pool)
	}
	return pdb, func() { pdb.Close() }
}
