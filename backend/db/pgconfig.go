package db

import (
	"context"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuerierFactory[T any] struct {
	FromPool func(pool *pgxpool.Pool) T
	FromTx   func(tx pgx.Tx) T
}

var PrimaryQuerierFactory = QuerierFactory[sqlc.Querier]{
	FromPool: func(pool *pgxpool.Pool) sqlc.Querier {
		return sqlc.New(pool)
	},
	FromTx: func(tx pgx.Tx) sqlc.Querier {
		return sqlc.New(tx)
	},
}

type PoolConfig struct {
	// (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	Dsn string `json:"dsn"`
	// (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	ActiveProfile config.Profile `json:"activeProfile"`
	// (optional) AWS region database is in, if AWS authentication is on
	Region string `json:"region"`
	// (optional) query to send to test connectivity, defaults to "SELECT 1"
	InitQuery string `json:"initQuery"`
}

func NewPostgresPool(ctx context.Context, cfg PoolConfig) *pgxpool.Pool {
	if cfg.InitQuery == "" {
		cfg.InitQuery = "SELECT 1"
	}

	slog.Info("creating postgres db pool", "config", cfg)

	poolCfg, err := pgxpool.ParseConfig(cfg.Dsn)
	if err != nil {
		logutil.Fatal("parse postgres config", err)
	}

	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second

	if cfg.ActiveProfile != config.Local {
		poolCfg.BeforeConnect = NewPgBeforeConnect(ctx, poolCfg.ConnConfig, cfg.Region)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create postgres pool", err)
	}
	if _, err = pool.Exec(ctx, cfg.InitQuery); err != nil {
		logutil.Fatal("execute postgres startup query", err)
	}

	slog.Info("connected to postgres successfully")
	return pool
}

func NewPostgresDB(ctx context.Context, cfg PoolConfig) Database[sqlc.Querier] {
	pool := NewPostgresPool(ctx, cfg)
	return &implDB[sqlc.Querier]{
		pool:    pool,
		factory: PrimaryQuerierFactory,
	}
}

func PostgresDBFromPool(pool *pgxpool.Pool) Database[sqlc.Querier] {
	return &implDB[sqlc.Querier]{
		pool:    pool,
		factory: PrimaryQuerierFactory,
	}
}

func NewFakePostgresDB(t logutil.TestLogger, pool *pgxpool.Pool) Database[sqlc.Querier] {
	testTx, err := pool.BeginTx(t.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("failed to begin primary test txn: %v", err)
	}
	return &fakeDB[sqlc.Querier]{
		testTxn: testTx,
		factory: PrimaryQuerierFactory,
	}
}
