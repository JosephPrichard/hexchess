package db

import (
	"context"
	"hexchess-svc/db/sqlc"
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

type PostgresDB struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func (pdb *PostgresDB) Querier() sqlc.Querier {
	return pdb.q
}

func (pdb *PostgresDB) Close() {
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
	return &PostgresDB{q: sqlc.New(pool), pool: pool}
}

func MakeFakeDB(txn pgx.Tx) DB {
	return &FakeDB{testingTxn: txn}
}
