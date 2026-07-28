package db

import (
	"context"
	"hexchess-svc/db/query"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryFactory[Querier any, ReadQuerier any] struct {
	Querier     func(pool *pgxpool.Pool) Querier
	ReadQuerier func(pool *pgxpool.Pool) ReadQuerier
	TxnQuerier  func(tx pgx.Tx) Querier
}

var PrimaryQuerierFactory = QueryFactory[ReadWriteQuerier, query.Querier]{
	Querier: func(pool *pgxpool.Pool) ReadWriteQuerier {
		return NewRWQuerier(pool)
	},
	ReadQuerier: func(pool *pgxpool.Pool) query.Querier {
		return NewROQuerier(pool)
	},
	TxnQuerier: func(tx pgx.Tx) ReadWriteQuerier {
		return NewRWQuerier(tx)
	},
}

type PoolConfig struct {
	Dsn           string         `json:"dsn"`           // (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	ActiveProfile config.Profile `json:"activeProfile"` // (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	AwsRegion     string         `json:"awsregion"`     // (optional) AWS region database is in, if AWS authentication is on
	InitQuery     string         `json:"initQuery"`     // (optional) query to send to test connectivity, defaults to "SELECT 1"
}

func NewDatabasePool(ctx context.Context, cfg PoolConfig) *pgxpool.Pool {
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
		poolCfg.BeforeConnect = NewPgBeforeConnect(ctx, poolCfg.ConnConfig, cfg.AwsRegion)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create postgres pool", err)
	}
	if _, err = pool.Exec(ctx, cfg.InitQuery); err != nil {
		logutil.Fatal("execute postgres startup query", err)
	}

	slog.Info("connected to database successfully", "dsn", cfg.Dsn)
	return pool
}

type DatabaseConfig struct {
	Dsn           string         `json:"dsn"`           // (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	ReadOnlyDsn   string         `json:"readOnlyDsn"`   // (optional) dsn for the read replicas of the postgres backend, defaults to Dsn if left empty
	ActiveProfile config.Profile `json:"activeProfile"` // (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	Region        string         `json:"region"`        // (optional) AWS region database is in, if AWS authentication is on
}

func NewDatabase(ctx context.Context, cfg DatabaseConfig) Database {
	return &implDB{
		writePool: NewDatabasePool(ctx, PoolConfig{
			Dsn:           cfg.Dsn,
			ActiveProfile: cfg.ActiveProfile,
			AwsRegion:     cfg.Region,
		}),
		readPool: NewDatabasePool(ctx, PoolConfig{
			Dsn:           cfg.ReadOnlyDsn,
			ActiveProfile: cfg.ActiveProfile,
			AwsRegion:     cfg.Region,
		}),
		factory: PrimaryQuerierFactory,
	}
}

func NewDatabaseFromPool(pool *pgxpool.Pool) Database {
	return &implDB{
		writePool: pool,
		readPool:  pool,
		factory:   PrimaryQuerierFactory,
	}
}

func NewFakeDatabase(t logutil.TestLogger, pool *pgxpool.Pool) Database {
	testTx, err := pool.BeginTx(t.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("failed to begin primary test txn: %v", err)
	}
	return &fakeDB{
		testTxn: testTx,
		factory: PrimaryQuerierFactory,
	}
}
