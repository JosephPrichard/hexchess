package db

import (
	"context"
	"hexchess-svc/db/metricsdb"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuerierFactory[T any] struct {
	FromPool func(pool *pgxpool.Pool) T
	FromTx   func(tx pgx.Tx) T
}

var PrimaryQuerierFactory = QuerierFactory[primarydb.Querier]{
	FromPool: func(pool *pgxpool.Pool) primarydb.Querier {
		return primarydb.New(pool)
	},
	FromTx: func(tx pgx.Tx) primarydb.Querier {
		return primarydb.New(tx)
	},
}

var MetricsQuerierFactory = QuerierFactory[metricsdb.Querier]{
	FromPool: func(pool *pgxpool.Pool) metricsdb.Querier {
		return metricsdb.New(pool)
	},
	FromTx: func(tx pgx.Tx) metricsdb.Querier {
		return metricsdb.New(tx)
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

	slog.Info("created postgres db pool")

	return pool
}

func NewPostgresDB[Querier any](ctx context.Context, factory QuerierFactory[Querier], cfg PoolConfig) Database[Querier] {
	pool := NewPostgresPool(ctx, cfg)
	return &implDB[Querier]{pool: pool, factory: factory}
}

func NewPrimaryDB(pool *pgxpool.Pool) Database[primarydb.Querier] {
	return &implDB[primarydb.Querier]{
		pool:    pool,
		factory: PrimaryQuerierFactory,
	}
}

func NewFakePrimaryDB(t logutil.TestLogger, pool *pgxpool.Pool) Database[primarydb.Querier] {
	testTx, err := pool.BeginTx(t.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("failed to begin primary test txn: %v", err)
	}
	return &fakeDB[primarydb.Querier]{
		testTxn: testTx,
		factory: PrimaryQuerierFactory,
	}
}

func NewMetricsDB(pool *pgxpool.Pool) Database[metricsdb.Querier] {
	return &implDB[metricsdb.Querier]{
		pool:    pool,
		factory: MetricsQuerierFactory,
	}
}

func NewFakeMetricsDB(t logutil.TestLogger, pool *pgxpool.Pool) Database[metricsdb.Querier] {
	testTx, err := pool.BeginTx(t.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("failed to begin metrics test txn: %v", err)
	}
	return &fakeDB[metricsdb.Querier]{
		testTxn: testTx,
		factory: MetricsQuerierFactory,
	}
}
