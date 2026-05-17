package itest

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"sync"
	"time"

	"hexchess-svc/db"
	"hexchess-svc/lib/logutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const RedisContTag = "redis:7.4.0"
const RedisContPort = "6379/tcp"

var muRedis sync.Mutex
var redisCont testcontainers.Container

func SetupRedisTest(ctx context.Context, t logutil.TestLogger) (rdb db.Redis, err error) {
	muRedis.Lock()
	defer muRedis.Unlock()

	if redisCont == nil {
		start := time.Now()
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        RedisContTag,
				ExposedPorts: []string{RedisContPort},
				Env:          map[string]string{},
				WaitingFor:   wait.ForListeningPort(RedisContPort),
			},
		})
		if err != nil {
			return rdb, fmt.Errorf("failed to start redis container: %w", err)
		}
		redisCont = cont
		t.Logf("finished starting redis container in %v", time.Since(start))
	}

	host, _ := redisCont.Host(ctx)
	port, _ := redisCont.MappedPort(ctx, RedisContPort)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	return db.MakeRedis(
		db.RedisAddrs{
			GameStoreAddr: addr,
			CacheAddr:     addr,
			PubsubAddr:    addr,
		},
		db.MakeTestRedisNames(),
	), nil
}

const PostgresContTag = "postgres:17"
const PgContPort = "5432/tcp"

const DbUser = "postgres"
const DbName = "postgres"
const DbPass = "postgres"

var muPostgres sync.Mutex
var postgresCont testcontainers.Container

func SetupPostgresTest(ctx context.Context, t logutil.TestLogger, testingTx bool) (pdb db.DB, err error) {
	muPostgres.Lock()
	defer muPostgres.Unlock()

	createdContainer := false

	if postgresCont == nil {
		start := time.Now()
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        PostgresContTag,
				ExposedPorts: []string{PgContPort},
				Env: map[string]string{
					"POSTGRES_USER":     DbUser,
					"POSTGRES_PASSWORD": DbPass,
					"POSTGRES_DB":       DbName,
				},
				WaitingFor: wait.ForListeningPort(PgContPort),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to start postgres container: %w", err)
		}
		postgresCont = cont
		createdContainer = true
		t.Logf("finished starting postgres container in %v", time.Since(start))
	}

	host, _ := postgresCont.Host(ctx)
	port, _ := postgresCont.MappedPort(ctx, PgContPort)
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", DbUser, DbPass, host, port.Port(), DbName)

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx conn: %w", err)
	}
	if createdContainer {
		if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
			return nil, fmt.Errorf("failed to reset schema: %w", err)
		}
		if _, err := pool.Exec(ctx, db.CreateSchema); err != nil {
			return nil, fmt.Errorf("failed to create schema: %w", err)
		}
		if err := insertTestData(pool); err != nil {
			return nil, fmt.Errorf("failed to insert test data: %w", err)
		}
	}

	if testingTx {
		testTx, err := pool.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: pgx.Serializable,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open testing txn: %w", err)
		}
		pdb = db.MakeFakeDB(testTx)
	} else {
		pdb = db.MakeDB(pool)
	}

	return pdb, nil
}
