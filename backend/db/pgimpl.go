package db

import (
	"context"
	"errors"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type implDB[Querier any] struct {
	factory   QuerierFactory[Querier]
	pool      *pgxpool.Pool
	refresher *PostgresTokenRefresher
}

func (db *fakeDB[Querier]) ExecTx(ctx context.Context, args TxArgs[Querier]) (err error) {
	// a fake postgres instance is already running in a txn, noop the txn
	return args.QueryFn(ctx, db.factory.FromTx(db.testTxn))
}

func (db *implDB[Querier]) Querier() Querier {
	return db.factory.FromPool(db.pool)
}

func (db *implDB[Querier]) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.refresher != nil {
		db.refresher.Shutdown()
	}
}

type fakeDB[Querier any] struct {
	testTxn pgx.Tx
	pool    *pgxpool.Pool
	factory QuerierFactory[Querier]
}

func (db *fakeDB[Querier]) Querier() Querier {
	return db.factory.FromTx(db.testTxn)
}

func (db *fakeDB[_]) Close() {
	if db.testTxn != nil {
		if err := db.testTxn.Rollback(context.Background()); err != nil {
			slog.Error("failed to rollback testing txn", "error", err)
		}
	}
	if db.pool != nil {
		db.pool.Close()
	}
}

func (db *implDB[Querier]) ExecTx(ctx context.Context, args TxArgs[Querier]) error {
	execTx := func(ctx context.Context, args TxArgs[Querier]) (txnErr error) {
		if args.Isolation == "" {
			args.Isolation = pgx.ReadCommitted
		}
		txn, txnErr := db.pool.BeginTx(ctx, pgx.TxOptions{
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

		txnErr = args.QueryFn(ctx, db.factory.FromTx(txn))
		return txnErr
	}

	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTx(ctx, args)

		if isSerializationFailure(err) {
			slog.WarnContext(ctx, "retrying transaction", "error", err, "retry", i)
			timeutil.Sleep(i, 2, 50*time.Millisecond)
			continue
		}
		break
	}
	if isSerializationFailure(err) {
		slog.ErrorContext(ctx, "exhausted transaction retries", "error", err, "retryCount", args.RetryCount)
	}

	return err
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
