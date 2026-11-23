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

type PostgreSQL struct {
	pools DatabasePools

	QuerierMutator QuerierMutator
	Querier        query.Querier
}

type Transactor interface {
	ExecTx(ctx context.Context, args TxArgs) error
}

func NewDatabase(pools DatabasePools) PostgreSQL {
	return PostgreSQL{pools: pools, QuerierMutator: NewPoolQuerier(pools.Write), Querier: NewPoolQuerier(pools.Read)}
}

func (db *PostgreSQL) HealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pools.Write)
}

func (db *PostgreSQL) ReadHealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pools.Read)
}

func (db *PostgreSQL) Close() {
	if db.pools.Write != nil {
		db.pools.Write.Close()
	}
	if db.pools.Read != nil {
		db.pools.Read.Close()
	}
	slog.Info("closed postgres database clients")
}

type TxArgs struct {
	QueryFn      func(context.Context, pgx.Tx, QuerierMutator) error
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}

func (db *PostgreSQL) ExecTx(ctx context.Context, args TxArgs) error {
	return execTx(ctx, db, args)
}

func execTx(ctx context.Context, db *PostgreSQL, args TxArgs) error {
	if args.Isolation == "" {
		args.Isolation = pgx.Serializable
	}
	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var execErr error

	for i := range args.RetryCount {
		execErr = execTxnOnce(ctx, db.pools.Write, args)
		if !IsSerializationFailure(execErr) {
			break
		}

		slog.WarnContext(ctx, "retrying transaction", "error", execErr, "retry", i)

		select {
		case <-ctx.Done():
			execErr = ctx.Err()
			break
		case <-time.After(timeutil.Backoff(i, 2, 50*time.Millisecond)):
		}
	}
	if IsSerializationFailure(execErr) {
		slog.ErrorContext(ctx, "exhausted transaction retries", "error", execErr, "retryCount", args.RetryCount)
	}

	return execErr
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

		doCommit := txnErr == nil || isAllowlisted(txnErr, args.ErrAllowlist)

		// END: after every transaction, we MUST EITHER commit OR rollback
		if doCommit {
			slog.InfoContext(ctx, "DB: commiting txn", "error", txnErr)

			if err := txn.Commit(ctx); err != nil {
				slog.ErrorContext(ctx, "DB: failed to commit txn", "error", err)
				txnErr = err
			}
		} else {
			slog.InfoContext(ctx, "DB: rolling back txn", "error", txnErr)

			if err := txn.Rollback(context.Background()); err != nil {
				slog.ErrorContext(ctx, "DB: failed to rollback txn", "error", err)
				txnErr = err
			}
		}
	}()

	return args.QueryFn(ctx, txn, NewTxnQuerier(txn))
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
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == ErrPgSerializationFailure || pgErr.Code == ErrPgDeadlock
	}
	return false
}
