package db

import (
	"context"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5"
)

type Database interface {
	ExecTx(context.Context, TxArgs[ReadWriteQuerier]) error
	Querier() ReadWriteQuerier
	ReadQuerier() query.Querier
	HealthcheckFunc() health.CheckFunc
	ReadHealthcheckFunc() health.CheckFunc
	Close()
}

type QueryFn[Querier any] func(context.Context, pgx.Tx, Querier) error

type TxArgs[Querier any] struct {
	QueryFn      QueryFn[Querier]
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

func NewRWQuerier(db mutator.DBTX) ReadWriteQuerier {
	return &ReadWriteQueries{
		ReadQueries:  query.New(db),
		WriteQueries: mutator.New(db),
	}
}

func NewROQuerier(db mutator.DBTX) ReadQuerier {
	return query.New(db)
}
