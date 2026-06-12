package itest

import (
	"context"
	"fmt"
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

func SetupRedisTest(ctx context.Context, t logutil.TestLogger) (string, error) {
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
			return "", fmt.Errorf("failed to start redis container: %w", err)
		}
		redisCont = cont
		t.Logf("finished starting redis container in %v", time.Since(start))
	}

	host, _ := redisCont.Host(ctx)
	port, _ := redisCont.MappedPort(ctx, RedisContPort)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	return addr, nil
}

const PostgresContTag = "postgres:17"
const PgContPort = "5432/tcp"

const DbUser = "postgres"
const DbName = "postgres"
const DbPass = "postgres"

var muPostgres sync.Mutex
var postgresCont testcontainers.Container

func SetupPostgresTest(ctx context.Context, t logutil.TestLogger) (*pgxpool.Pool, error) {
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

	return pool, nil
}

const LocalstackContTag = "localstack/localstack:3.0"
const LocalstackContPort = "4566/tcp"

var muLocalstack sync.Mutex
var localstackCont testcontainers.Container

func SetupLocalstackTest(ctx context.Context, t logutil.TestLogger) (string, error) {
	muLocalstack.Lock()
	defer muLocalstack.Unlock()

	if localstackCont == nil {
		start := time.Now()
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        LocalstackContTag,
				ExposedPorts: []string{LocalstackContPort},
				WaitingFor:   wait.ForListeningPort(LocalstackContPort),
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to start localstack container: %w", err)
		}
		localstackCont = cont
		t.Logf("finished starting localstack container in %v", time.Since(start))
	}

	host, _ := localstackCont.Host(ctx)
	port, _ := localstackCont.MappedPort(ctx, LocalstackContPort)
	addr := fmt.Sprintf("http://%s:%s", host, port.Port())

	return addr, nil
}
