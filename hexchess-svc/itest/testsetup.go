package itest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/pkg/logutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
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

	return db.MakeRdb(
		db.RedisAddrs{CacheAddr: addr, PubsubAddr: addr},
		db.RedisNames{
			LeaderboardZSet:  db.LeaderboardZSet + "_" + uuid.NewString(),
			GamesZSet:        db.GamesZSet + "_" + uuid.NewString(),
			ActiveUsersZSet:  db.ActiveUsersZSet + "_" + uuid.NewString(),
			GameChatsPostfix: db.GameChatsPrefix + "_" + uuid.NewString(),
			GamesChan:        db.GamesChan + "_" + uuid.NewString(),
			UsersChan:        db.UsersChan + "_" + uuid.NewString(),
			GamesCountChan:   db.GamesCountChan + "_" + uuid.NewString(),
			ActiveCountChan:  db.ActiveCountChan + "_" + uuid.NewString(),
		},
	), nil
}

const PostgresContTag = "postgres:17"
const PgContPort = "5432/tcp"

const DbUser = "postgres"
const DbName = "postgres"
const DbPass = "postgres"

var muPostgres sync.Mutex
var postgresCont testcontainers.Container

func SetupPostgresTest(ctx context.Context, t logutil.TestLogger, testingTx bool) (pdb db.Postgres, err error) {
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
		insertTestData(t, pool)
	}

	if testingTx {
		testTx, err := pool.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open testing txn: %w", err)
		}
		pdb = db.MakeFakePostgres(testTx)
	} else {
		pdb = db.MakePostgres(pool)
	}

	return pdb, nil
}

const LocalstackContTag = "localstack/localstack:3.0.2"
const LocalStackContPort = "4566/tcp"

var muLocalstack sync.Mutex
var localstackCont testcontainers.Container

func SetupAwsTest(ctx context.Context, t logutil.TestLogger) (awsClient egress.Aws, err error) {
	muLocalstack.Lock()
	defer muLocalstack.Unlock()

	if localstackCont == nil {
		start := time.Now()
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        LocalstackContTag,
				ExposedPorts: []string{LocalStackContPort},
				WaitingFor:   wait.ForListeningPort(LocalStackContPort),
			},
		})
		if err != nil {
			return awsClient, fmt.Errorf("start localstack container: %s", err)
		}
		localstackCont = cont
		t.Logf("finished starting localstack container in %v", time.Since(start))
	}

	host, _ := localstackCont.Host(ctx)
	port, _ := localstackCont.MappedPort(ctx, LocalStackContPort)
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	cfg := egress.AwsConfig{
		S3ProfileBucket:  egress.S3ProfileBucket + "-" + uuid.NewString(),
		AwsDefaultRegion: "us-east-1",
		AwsSecretKey:     "testing",
		AwsSecretID:      "testing",
		AwsEndpoint:      endpoint,
	}
	awsClients, err := egress.MakeAwsClients(ctx, cfg)
	if err != nil {
		return awsClient, fmt.Errorf("make aws clients: %s", err)
	}

	for _, bucket := range []string{
		cfg.S3ProfileBucket,
	} {
		if _, err := awsClients.S3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
			return awsClient, fmt.Errorf("create s3 bucket: %v: %s", bucket, err)
		}
	}
	return awsClients, nil
}
