package database

import (
	"context"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/config"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type PoolConfig struct {
	Dsn           string         `json:"dsn"`           // (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	ActiveProfile config.Profile `json:"activeProfile"` // (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	AwsRegion     string         `json:"awsregion"`     // (opt) AWS region database is in, if AWS authentication is on
	InitQuery     string         `json:"initQuery"`     // (opt) query to send to test connectivity, defaults to "SELECT 1"
}

func NewDatabasePool(ctx context.Context, cfg PoolConfig) *pgxpool.Pool {
	if cfg.InitQuery == "" {
		cfg.InitQuery = "SELECT 1"
	}

	slog.Info("creating postgres database pool", "config", cfg)

	poolCfg, err := pgxpool.ParseConfig(cfg.Dsn)
	if err != nil {
		alog.Fatal("parse postgres config", err, "config", cfg)
	}

	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second

	if cfg.ActiveProfile != config.Local {
		poolCfg.BeforeConnect = NewPgBeforeConnect(ctx, poolCfg.ConnConfig, cfg.AwsRegion)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		alog.Fatal("create postgres pool", err, "config", cfg)
	}
	if _, err = pool.Exec(ctx, cfg.InitQuery); err != nil {
		alog.Fatal("execute postgres startup query", err, "config", cfg)
	}

	slog.Info("connected to database successfully", "config", cfg)
	return pool
}

type DatabaseConfig struct {
	ReadWriteDsn  string         `json:"readWriteDsn"`  // (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	ReadDsn       string         `json:"readDsn"`       // (opt) dsn for the read replicas of the postgres backend, reuses the read write pool if left empty
	ActiveProfile config.Profile `json:"activeProfile"` // (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	AwsRegion     string         `json:"awsRegion"`     // (opt) AWS region database is in, if AWS authentication is on
}

func NewDatabase(ctx context.Context, cfg DatabaseConfig) Database {
	writePool := NewDatabasePool(ctx, PoolConfig{Dsn: cfg.ReadWriteDsn, ActiveProfile: cfg.ActiveProfile, AwsRegion: cfg.AwsRegion})

	var readPool *pgxpool.Pool
	if cfg.ReadDsn != "" {
		readPool = NewDatabasePool(ctx, PoolConfig{Dsn: cfg.ReadDsn, ActiveProfile: cfg.ActiveProfile, AwsRegion: cfg.AwsRegion})
	} else {
		readPool = writePool
	}

	return Database{kind: realDatabase, writePool: writePool, readPool: readPool}
}

func NewDatabaseFromPool(pool *pgxpool.Pool) Database {
	return Database{
		kind:      realDatabase,
		writePool: pool,
		readPool:  pool,
	}
}

func NewFakeDatabase(t alog.TestLogger, pool *pgxpool.Pool) Database {
	testTx, err := pool.BeginTx(t.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatalf("failed to begin primary test txn: %v", err)
	}
	return Database{
		kind:      fakeDatabase,
		testTxn:   testTx,
		writePool: pool,
		readPool:  pool,
	}
}

func NewRiverClient(ctx context.Context, cfg PoolConfig) RiverClientAPI {
	pool := NewDatabasePool(ctx, PoolConfig{Dsn: cfg.Dsn, ActiveProfile: cfg.ActiveProfile, AwsRegion: cfg.AwsRegion})
	return NewRiverClientFromPool(pool)
}

func NewRiverClientFromPool(pool *pgxpool.Pool) RiverClientAPI {
	riverProducerClient, err := river.NewClient(riverpgxv5.New(pool), nil)
	if err != nil {
		alog.Fatal("create river queue client", err)
	}
	return riverProducerClient
}
