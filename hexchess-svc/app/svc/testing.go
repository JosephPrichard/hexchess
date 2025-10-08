package svc

import (
	"context"
	"fmt"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/db"
	"testing"
	"time"
)

const TestDbUser = "postgres"
const TestDbName = "hexchess"
const TestDbPass = "password123"
const TestDbPort = 9876

func createEmbeddedDb(t *testing.T) func() {
	config := embeddedpostgres.DefaultConfig().
		Username(TestDbUser).
		Password(TestDbPass).
		Database(TestDbName).
		Version(embeddedpostgres.V15).
		RuntimePath("/tmp").
		Port(uint32(TestDbPort)).
		StartTimeout(30 * time.Second)
	t.Logf("starting the embedded test database with config.go: %v", config)

	postgres := embeddedpostgres.NewDatabase(config)
	err := postgres.Start()
	if err != nil {
		t.Fatalf("failed to start embedded datanase: %v", err)
	}

	t.Logf("finished setting up the embedded test database")

	closer := func() {
		err := postgres.Stop()
		if err != nil {
			t.Fatalf("failed to stop test db with err: %v", err)
		}
	}
	return closer
}

func makeTestPgxPool(t *testing.T) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%d", TestDbUser, TestDbName, TestDbPass, TestDbPort))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	return pool
}

func resetTestSchema(t *testing.T, pool *pgxpool.Pool) {
	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;"); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	if _, err := pool.Exec(context.Background(), db.CreateSchema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
}
