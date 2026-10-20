package database

import (
	"context"
	"errors"
	"hexchess-svc/database/mutate"
	"hexchess-svc/database/query"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"time"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	Mutator    mutate.Querier
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

type RiverClientAPI interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Stop(ctx context.Context) error
}

func (db *Database) QuerierMutator() QuerierMutator {
	if db.querierMutator != nil {
		return db.querierMutator
	}
	db.querierMutator = NewPoolQuerier(db.pools.Write)
	return db.querierMutator
}

func (db *Database) Mutator() mutate.Querier {
	return db.QuerierMutator()
}

func (db *Database) Querier() query.Querier {
	if db.querier != nil {
		return db.querier
	}
	db.querier = NewPoolQuerier(db.pools.Read)
	return db.querier
}

func (db *Database) HealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pools.Write)
}

func (db *Database) ReadHealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pools.Read)
}

func (db *Database) Close() {
	if db.pools.Write != nil {
		db.pools.Write.Close()
	}
	if db.pools.Read != nil {
		db.pools.Read.Close()
	}
}

func (db *Database) ExecTx(ctx context.Context, args TxArgs) error {
	return execTx(ctx, db, args)
}

func execTx(ctx context.Context, db *Database, args TxArgs) error {
	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTxnOnce(ctx, db.pools.Write, args)

		if isSerializationFailure(err) {
			slog.WarnContext(ctx, "retrying transaction", "error", err, "retry", i)
			timeutil.BackoffSleep(i, 2, 50*time.Millisecond)
			continue
		}
		break
	}
	if isSerializationFailure(err) {
		slog.ErrorContext(ctx, "exhausted transaction retries", "error", err, "retryCount", args.RetryCount)
	}

	return err
}

func execTxnOnce(ctx context.Context, pool *pgxpool.Pool, args TxArgs) (txnErr error) {
	if args.Isolation == "" {
		args.Isolation = pgx.ReadCommitted
	}
	txn, txnErr := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: args.Isolation,
	})
	if txnErr != nil {
		return txnErr
	}

	defer func() {
		if panicErr := recover(); panicErr != nil {
			_ = txn.Rollback(ctx) // best-effort; we're already panicking
			panic(panicErr)
		}

		if txnErr != nil && !isAllowlisted(txnErr, args.ErrAllowlist) {
			if err := txn.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback txn", "error", err)
				txnErr = err
			}
			return
		}

		if err := txn.Commit(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to commit txn", "error", err)
			txnErr = err
		}
	}()

	txnErr = args.QueryFn(ctx, txn, NewTxnQuerier(txn))
	return txnErr
}

func isAllowlisted(err error, allowlist []error) bool {
	for _, a := range allowlist {
		if errors.Is(err, a) {
			return true
		}
	}
	return false
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == ErrPgSerializationFailure || pgErr.Code == ErrPgDeadlock)
}
