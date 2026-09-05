package itest

import (
	"context"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Test Preconditions: infrastructure is already running at these addresses (use docker compose in root)
	redisAddr      = "localhost:20121"
	localstackAddr = "http://localhost:30121"
	dbAddr         = "localhost:40121"

	dbUser = "postgres"
	dbName = "postgres"
	dbPass = "postgres"
)

type TestInfra struct {
	database.Database
	Redis cache.Redis
	AWS   cloud.AWSClient
}

func (i TestInfra) Close() {
	i.Redis.Close()
	i.Database.Close()
}

func SetupIntegrationTest(t slogutil.TestLogger) TestInfra {
	ctx := t.Context()

	pgPool := createPool(t)

	var infra TestInfra

	infra.Database = database.NewDatabaseFromPool(pgPool)

	infra.Redis = cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   []string{redisAddr},
		PubsubAddr:    redisAddr,
		ActiveProfile: config.Local,
	})

	infra.AWS = cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		AWSEndpoint:   localstackAddr,
		AWSRegion:     "us-east-1",
		AWSUsername:   "testing",
		AWSPassword:   "testing",
		ActiveProfile: config.Local,
		Names:         *testutil.NewTestNames(cloud.DefaultAWSNames),
	})

	setupRedisPreconditions(t)
	setupDbPreconditions(t, pgPool)

	return infra
}

func createPool(t slogutil.TestLogger) *pgxpool.Pool {
	ctx := t.Context()
	connString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbUser, dbPass, dbAddr, dbName)

	pgPool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("failed to create pg pool: %v", err)
	}
	return pgPool
}

func setupDbPreconditions(t slogutil.TestLogger, pool *pgxpool.Pool) {
	ctx := context.WithValue(context.Background(), slogutil.Trace, "insert-testing-data")

	if err := dropSchema(ctx, pool); err != nil {
		t.Fatalf("failed to drop schema: %v", err)
	}
	if err := createSchema(ctx, pool); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	if err := seedDatabase(ctx, pool); err != nil {
		t.Fatalf("failed to seed database: %v", err)
	}
}

func dropSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	return err
}

func createSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, database.CreatePrimarySchema)
	return err
}

func setupRedisPreconditions(t slogutil.TestLogger) {
	pool := &redigo.Pool{
		MaxIdle:     8,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			return redigo.Dial("tcp", redisAddr)
		},
	}
	conn := pool.Get()
	defer conn.Close()

	if _, err := conn.Do("FLUSHALL"); err != nil {
		t.Fatalf("failed to flush redis: %v", err)
	}
}
