package database

import (
	"context"
	"errors"
	"hexchess-svc/database/mutator"
	"hexchess-svc/database/query"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"time"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (db *Database) QuerierMutator() QuerierMutator {
	switch db.kind {
	case realDatabase:
		return NewPoolQuerier(db.writePool)
	case fakeDatabase:
		return NewTxnQuerier(db.testTxn)
	}
	return nil
}

func (db *Database) Mutator() mutator.Querier {
	return db.QuerierMutator()
}

func (db *Database) Querier() query.Querier {
	switch db.kind {
	case realDatabase:
		return NewPoolQuerier(db.readPool)
	case fakeDatabase:
		return NewTxnQuerier(db.testTxn)
	}
	return nil
}

func (db *Database) HealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.writePool)
}

func (db *Database) ReadHealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.readPool)
}

func (db *Database) Close() {
	if db.testTxn != nil {
		// a fake postgres instance is running with a transaction, clean it up
		if err := db.testTxn.Rollback(context.Background()); err != nil {
			slog.Error("failed to rollback testing txn", "error", err)
		}
	}
	if db.writePool != nil {
		db.writePool.Close()
	}
	if db.readPool != nil {
		db.readPool.Close()
	}
}

func (db *Database) ExecTx(ctx context.Context, args TxArgs) error {
	switch db.kind {
	case realDatabase:
		return execTx(ctx, db, args)
	case fakeDatabase:
		// a fake postgres instance is already running in a txn, noop the txn
		return args.QueryFn(ctx, db.testTxn, NewTxnQuerier(db.testTxn))
	}
	return nil
}

func execTx(ctx context.Context, db *Database, args TxArgs) error {
	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTxnOnce(ctx, db.writePool, args)

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
