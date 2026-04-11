package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/db/sqlc"
)

type DB interface {
	Querier() sqlc.Querier
	ExecTx(context.Context, Tx) error
	Close()
}

type PostgresDB struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func (pdb *PostgresDB) Querier() sqlc.Querier {
	return pdb.q
}

func (pdb *PostgresDB) Close() {
	pdb.pool.Close()
}

type FakeDB struct {
	testingTxn pgx.Tx
}

func (pdb *FakeDB) Querier() sqlc.Querier {
	return sqlc.New(pdb.testingTxn)
}

func (pdb *FakeDB) Close() {
	if err := pdb.testingTxn.Rollback(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to rollback testing txn: %v", err))
	}
}

func MakeDB(pool *pgxpool.Pool) DB {
	return &PostgresDB{q: sqlc.New(pool), pool: pool}
}

func MakeFakeDB(txn pgx.Tx) DB {
	return &FakeDB{testingTxn: txn}
}
