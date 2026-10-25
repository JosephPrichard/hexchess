package database

import (
	"hexchess-svc/database/mutate"
	"hexchess-svc/database/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate mockgen -source=./querier.go -destination=./querier_mock.go -package=database

type QuerierMutator interface {
	query.Querier
	mutate.Querier
}

type Querier interface {
	query.Querier
}

type Mutator interface {
	mutate.Querier
}

type readQueries = query.Queries
type writeQueries = mutate.Queries

type ReadWriteQueries struct {
	*readQueries
	*writeQueries
}

func NewPoolQuerier(pool *pgxpool.Pool) QuerierMutator {
	if pool == nil {
		return nil
	}
	return ReadWriteQueries{readQueries: query.New(pool), writeQueries: mutate.New(pool)}
}

func NewTxnQuerier(txn pgx.Tx) QuerierMutator {
	if txn == nil {
		return nil
	}
	return ReadWriteQueries{readQueries: query.New(txn), writeQueries: mutate.New(txn)}
}
