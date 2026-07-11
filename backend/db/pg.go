package db

import (
	"context"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database interface {
	ExecTx(context.Context, TxArgs) error
	Querier() sqlc.Querier
	Close()
}

type ImplDB struct {
	q         *sqlc.Queries
	pool      *pgxpool.Pool
	refresher *PostgresTokenRefresher
}

func (pdb *ImplDB) Querier() sqlc.Querier {
	return pdb.q
}

func (pdb *ImplDB) Close() {
	if pdb.pool != nil {
		pdb.pool.Close()
	}
	if pdb.refresher != nil {
		pdb.refresher.Shutdown()
	}
}

type FakeDB struct {
	testingTxn pgx.Tx
}

func (pdb *FakeDB) Querier() sqlc.Querier {
	return sqlc.New(pdb.testingTxn)
}

func (pdb *FakeDB) Close() {
	if pdb.testingTxn == nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			slog.Error("fatal error while closing fake db", "error", p)
		}
	}()
	if err := pdb.testingTxn.Rollback(context.Background()); err != nil {
		slog.Error("failed to rollback testing txn", "error", err)
	}
}

type PgPoolConfig struct {
	// (required) parseable configuration in either KV pair or postgres URL format. see pgxpool documentation.
	Dsn string `json:"dsn"`
	// (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	ActiveProfile config.Profile `json:"activeProfile"`
	// (optional) AWS region database is in, if AWS authentication is on
	Region string `json:"region"`
	// (optional) query to send to test connectivity, defaults to "SELECT 1"
	InitQuery string `json:"initQuery"`
}

func NewPostgresDB(ctx context.Context, cfg PgPoolConfig) Database {
	slog.Info("creating postgres db client", "config", cfg)

	var refresher *PostgresTokenRefresher

	poolCfg, err := pgxpool.ParseConfig(cfg.Dsn)
	if err != nil {
		logutil.Fatal("parse postgres config", err)
	}

	if cfg.ActiveProfile != config.Local {
		refresher := NewPgTokenRefresher(ctx, cfg.Region)
		poolCfg.BeforeConnect = NewBeforeConnect(refresher)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create postgres pool", err)
	}
	if cfg.InitQuery == "" {
		cfg.InitQuery = "SELECT 1"
	}
	if _, err = pool.Exec(ctx, cfg.InitQuery); err != nil {
		logutil.Fatal("execute postgres startup query", err)
	}

	return &ImplDB{q: sqlc.New(pool), pool: pool, refresher: refresher}
}

func NewPostgresDBFromPool(pool *pgxpool.Pool) Database {
	return &ImplDB{q: sqlc.New(pool), pool: pool}
}

func NewFakeDB(txn pgx.Tx) Database {
	return &FakeDB{testingTxn: txn}
}
