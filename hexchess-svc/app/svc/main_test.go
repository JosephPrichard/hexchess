package svc

import (
	"context"
	"fmt"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"hexchess-svc/db"
	"log"
	"os"
	"testing"
	"time"
)

const TestDbUser = "postgres"
const TestDbName = "hexchess"
const TestDbPass = "password123"
const TestDbPort = 9876

var postgres *embeddedpostgres.EmbeddedPostgres
var redisCont *redis.RedisContainer

func TestMain(m *testing.M) {
	teardown := func() {
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

	var exitCode int
	func() {
		defer teardown()
		exitCode = m.Run()
	}()

	os.Exit(exitCode)
}

func setupEmbeddedDb(t *testing.T) {
	if postgres == nil {
		start := time.Now()

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

		t.Logf("finished setting up the embedded test database in %v", time.Now().Sub(start))
	}
}

func getRedisContainerAddr(t *testing.T) string {
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

func beforeDbTests(t *testing.T) (DB, func()) {
	setupEmbeddedDb(t)

	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%d", TestDbUser, TestDbName, TestDbPass, TestDbPort))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	closer := func() {
		pool.Close()
	}

	q := db.New(pool)
	pgDB := MakeDbClient(q, pool)

	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	if _, err := pool.Exec(context.Background(), db.CreateSchema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return pgDB, closer
}
