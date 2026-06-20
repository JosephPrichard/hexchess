package db

import (
	"context"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
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

func MakeDB(pool *pgxpool.Pool) DB {
	return &ImplDB{q: sqlc.New(pool), pool: pool}
}

func MakeFakeDB(txn pgx.Tx) DB {
	return &FakeDB{testingTxn: txn}
}

func MakePgPool(ctx context.Context, dsn string) *pgxpool.Pool {
	slog.Info("creating to postgres db client", "dbURL", dsn)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logutil.FatalErr("parse postgres config", err)
	}
	config.BeforeConnect = func(ctx context.Context, cfg *pgx.ConnConfig) error {
		slog.InfoContext(ctx, "before connecting to postgres")
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logutil.FatalErr("create postgres pool", err)
	}
	if _, err = pool.Exec(ctx, "SELECT 1;"); err != nil {
		logutil.FatalErr("execute postgres startup query", err)
	}

	slog.InfoContext(ctx, "created postgres db client", "dbURL", dsn, "config", config)

	return pool
}
