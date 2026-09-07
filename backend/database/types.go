package database

import (
	"context"
	"hexchess-svc/database/mutator"
	"hexchess-svc/database/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type Database struct {
	pools DatabasePools

	querierMutator QuerierMutator
	querier        query.Querier
}

type Queriers struct {
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

type RiverClientAPI interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Stop(ctx context.Context) error
}
