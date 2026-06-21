package db

import (
	"context"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database interface {
	Transactor
	Querier() sqlc.Querier
	Close()
}

type Transactor interface {
	ExecTx(context.Context, TxArgs) error
}

type ImplDB struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func (pdb *ImplDB) Querier() sqlc.Querier {
	return pdb.q
}

func (pdb *ImplDB) Close() {
	if pdb.pool != nil {
		pdb.pool.Close()
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

func NewDB(pool *pgxpool.Pool) Database {
	return &ImplDB{q: sqlc.New(pool), pool: pool}
}

func NewFakeDB(txn pgx.Tx) Database {
	return &FakeDB{testingTxn: txn}
}

type PgConnectCfg struct {
	Dsn           string `json:"dsn"`
	ActiveProfile string `json:"activeProfile"`
	Region        string `json:"region"`
}

func NewPgPool(ctx context.Context, cfg PgConnectCfg) (*pgxpool.Pool, func()) {
	slog.Info("creating to postgres db client", "cfg", cfg)

	poolCfg, err := pgxpool.ParseConfig(cfg.Dsn)
	if err != nil {
		logutil.Fatal("parse postgres config", err)
	}

	var connector *PGConnector
	if cfg.ActiveProfile != "local" {
		connector = NewPgConnector(ctx, cfg.Region)
		poolCfg.BeforeConnect = connector.BeforeConnect
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create postgres pool", err)
	}
	if _, err = pool.Exec(ctx, "SELECT 1;"); err != nil {
		logutil.Fatal("execute postgres startup query", err)
	}

	slog.InfoContext(ctx, "created postgres db client", "cfg", cfg)

	closer := func() {
		pool.Close()
		if connector != nil {
			connector.Stop()
		}
	}
	return pool, closer
}
