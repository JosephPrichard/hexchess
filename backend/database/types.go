package database

import (
	"context"
	"hexchess-svc/database/mutator"
	"hexchess-svc/database/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type databaseImplKind int

const (
	realDatabase databaseImplKind = iota
	fakeDatabase
)

type Database struct {
	kind      databaseImplKind
	writePool *pgxpool.Pool
	readPool  *pgxpool.Pool
	testTxn   pgx.Tx
}

type Operator struct {
	Transactor Transactor
	Mutator    mutator.Querier
	Querier    query.Querier
}

type Transactor interface {
	ExecTx(ctx context.Context, args TxArgs) error
}

type QueryFn func(context.Context, pgx.Tx, QuerierMutator) error

type TxArgs struct {
	QueryFn      QueryFn
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}
type QuerierMutator interface {
	query.Querier
	mutator.Querier
}

type readQueries = query.Queries
type writeQueries = mutator.Queries

type ReadWriteQueries struct {
	*readQueries
	*writeQueries
}

func NewPoolQuerier(pool *pgxpool.Pool) QuerierMutator {
	if pool == nil {
		return nil
	}
	return ReadWriteQueries{readQueries: query.New(pool), writeQueries: mutator.New(pool)}
}

func NewTxnQuerier(txn pgx.Tx) QuerierMutator {
	if txn == nil {
		return nil
	}
	return ReadWriteQueries{readQueries: query.New(txn), writeQueries: mutator.New(txn)}
}
