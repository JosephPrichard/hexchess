package database

import (
	"context"
	"errors"
	"hexchess-svc/database/query"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"time"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pools DatabasePools

	QuerierMutator QuerierMutator
	Querier        query.Querier
}

type Transactor interface {
	ExecTx(ctx context.Context, args TxArgs) error
}

func NewDatabase(pools DatabasePools) Database {
	return Database{pools: pools, QuerierMutator: NewPoolQuerier(pools.Write), Querier: NewPoolQuerier(pools.Read)}
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

type TxArgs struct {
	QueryFn      func(context.Context, pgx.Tx, QuerierMutator) error
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
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

		if IsSerializationFailure(err) {
			slog.WarnContext(ctx, "retrying transaction", "error", err, "retry", i)
			timeutil.BackoffSleep(i, 2, 50*time.Millisecond)
			continue
		}
		break
	}
	if IsSerializationFailure(err) {
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

func IsSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == ErrPgSerializationFailure || pgErr.Code == ErrPgDeadlock)
}
