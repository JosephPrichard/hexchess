package db

import (
	"context"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"

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

type Transactor interface {
	ExecTx(ctx context.Context, args TxArgs) error
}

type QueryFn func(context.Context, pgx.Tx, ReadWriteQuerier) error

type TxArgs struct {
	QueryFn      QueryFn
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}

type ReadQuerier = query.Querier
type WriteQuerier = mutator.Querier

type ReadWriteQuerier interface {
	query.Querier
	mutator.Querier
}

type ReadQueries = query.Queries
type WriteQueries = mutator.Queries

type ReadWriteQueries struct {
	*ReadQueries
	*WriteQueries
}

func NewPoolQuerier(pool *pgxpool.Pool) ReadWriteQuerier {
	if pool == nil {
		return nil
	}
	return ReadWriteQueries{
		ReadQueries:  query.New(pool),
		WriteQueries: mutator.New(pool),
	}
}

func NewTxnQuerier(txn pgx.Tx) ReadWriteQuerier {
	if txn == nil {
		return nil
	}
	return ReadWriteQueries{
		ReadQueries:  query.New(txn),
		WriteQueries: mutator.New(txn),
	}
}
