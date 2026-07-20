package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Database[Querier any] interface {
	ExecTx(context.Context, TxArgs[Querier]) error
	Querier() Querier
	Close()
}

type QueryFn[Querier any] func(ctx context.Context, querier Querier) error

type TxArgs[Querier any] struct {
	QueryFn      QueryFn[Querier]
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}
