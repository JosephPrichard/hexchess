package itest

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"hexchess-svc/db"
	"hexchess-svc/ext"
	"hexchess-svc/pkg/logutil"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const DbUser = "postgres"
const DbName = "postgres"
const DbPass = "postgres"

const LocalstackContTag = "localstack/localstack:3.0.2"
const RedisContTag = "redis:8.4.0"
const PostgresContTag = "postgres:17"

const LocalStackContPort = "4566/tcp"
const RedisContPort = "6379/tcp"
const PgContPort = "5432/tcp"

var muLocalstack sync.Mutex
var localstackCont testcontainers.Container

var muRedis sync.Mutex
var redisCont testcontainers.Container

var muPostgres sync.Mutex
var postgresCont testcontainers.Container

func GetRedisContainer(ctx context.Context, t logutil.TestLogger) string {
	muRedis.Lock()
	defer muRedis.Unlock()

	if redisCont == nil {
		start := time.Now()
		cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        RedisContTag,
				ExposedPorts: []string{RedisContPort},
				Env: map[string]string{
					"POSTGRES_USER":     DbUser,
					"POSTGRES_PASSWORD": DbPass,
					"POSTGRES_DB":       DbName,
				},
				WaitingFor: wait.ForListeningPort(RedisContPort),
			},
		})
		if err != nil {
			t.Fatalf("failed to start redis container: %s", err)
		}
		redisCont = cont
		t.Logf("finished starting redis container in %v", time.Since(start))
	}

	host, _ := redisCont.Host(ctx)
	port, _ := redisCont.MappedPort(ctx, RedisContPort)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	return addr
}

func SetupRedisTest(t logutil.TestLogger) db.Redis {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	addr := GetRedisContainer(ctx, t)

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
	)
}

func GetPgContainer(ctx context.Context, t logutil.TestLogger) (string, bool) {
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
			t.Fatalf("failed to start postgres container: %s", err)
		}
		postgresCont = cont
		createdContainer = true
		t.Logf("finished starting postgres container in %v", time.Since(start))
	}

	host, _ := postgresCont.Host(ctx)
	port, _ := postgresCont.MappedPort(ctx, PgContPort)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", DbUser, DbPass, host, port.Port(), DbName), createdContainer
}

func SetupPostgresTest(t logutil.TestLogger, useTestTx bool) db.Postgres {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	connString, shouldSeed := GetPgContainer(ctx, t)

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("failed to create pgx conn: %v", err)
	}
	if shouldSeed {
		// seed logic can run outside the lock - the lock will return the `shouldSeed` flag is true once, as it is derived from the locked data.
		if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
			t.Fatalf("failed to reset schema: %v", err)
		}
		if _, err := pool.Exec(ctx, db.CreateSchema); err != nil {
			t.Fatalf("failed to create schema: %v", err)
		}
		insertTestData(t, pool)
	}

	var pdb db.Postgres
	if useTestTx {
		testTx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to open testing txn: %v", err)
		}
		pdb = db.MakeFakePostgres(testTx)
	} else {
		pdb = db.MakePostgres(pool)
	}

	return pdb
}

func SetupAwsTest(t logutil.TestLogger) ext.Aws {
	muLocalstack.Lock()
	defer muLocalstack.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Hour)
	defer cancel()

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
			t.Fatalf("start localstack container: %s", err)
		}
		localstackCont = cont
		t.Logf("finished starting localstack container in %v", time.Since(start))
	}

	host, _ := localstackCont.Host(ctx)
	port, _ := localstackCont.MappedPort(ctx, LocalStackContPort)
	addr := fmt.Sprintf("http://%s:%s", host, port.Port())

	awsClients, err := ext.MakeAwsClients(ctx, ext.AwsConfig{
		S3Bucket:         ext.S3Bucket + uuid.NewString(),
		AwsDefaultRegion: "us-east-1",
		AwsSecretKey:     "testing",
		AwsSecretID:      "testing",
		AwsEndpoint:      addr,
	})
	if err != nil {
		t.Fatalf("make aws clients: %s", err)
	}

	if _, err := awsClients.S3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(awsClients.S3Bucket),
	}); err != nil {
		t.Fatalf("create s3 bucket: %v: %s", awsClients.S3Bucket, err)
	}
	return awsClients
}
