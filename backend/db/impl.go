package db

import (
	"context"
	"errors"
	"hexchess-svc/db/query"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"time"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryFactoryImpl = QueryFactory[ReadWriteQuerier, query.Querier]

type fakeDB struct {
	testTxn pgx.Tx
	pool    *pgxpool.Pool
	factory QueryFactoryImpl
}

func (db *fakeDB) Querier() ReadWriteQuerier {
	return db.factory.TxnQuerier(db.testTxn)
}

func (db *fakeDB) ReadQuerier() query.Querier {
	return db.Querier() // read querier is the same as the read/write querier functional tests, it is for perf only
}

func (db *fakeDB) HealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pool)
}

func (db *fakeDB) ReadHealthcheckFunc() health.CheckFunc {
	return NewHealthcheck(db.pool)
}

func (db *fakeDB) ExecTx(ctx context.Context, args TxArgs[ReadWriteQuerier]) (err error) {
	// a fake postgres instance is already running in a txn, noop the txn
	return args.QueryFn(ctx, db.testTxn, db.factory.TxnQuerier(db.testTxn))
}

func (db *fakeDB) Close() {
	if db.testTxn != nil {
		if err := db.testTxn.Rollback(context.Background()); err != nil {
			slog.Error("failed to rollback testing txn", "error", err)
		}
	}
	if db.pool != nil {
		db.pool.Close()
	}
}

type implDB struct {
	factory   QueryFactory[ReadWriteQuerier, query.Querier]
	writePool *pgxpool.Pool
	readPool  *pgxpool.Pool
}

func (db *implDB) Querier() ReadWriteQuerier {
	if db.writePool == nil {
		return nil
	}
	return db.factory.Querier(db.writePool)
}

func (db *implDB) ReadQuerier() query.Querier {
	if db.readPool == nil {
		return nil
	}
	return db.factory.Querier(db.readPool)
}

func (db *implDB) HealthcheckFunc() health.CheckFunc {
	if db.writePool == nil {
		return nil
	}
	return NewHealthcheck(db.writePool)
}

func (db *implDB) ReadHealthcheckFunc() health.CheckFunc {
	if db.readPool == nil {
		return nil
	}
	return NewHealthcheck(db.readPool)
}

func (db *implDB) Close() {
	if db.writePool != nil {
		db.writePool.Close()
	}
	if db.readPool != nil {
		db.readPool.Close()
	}
}

func (db *implDB) ExecTx(ctx context.Context, args TxArgs[ReadWriteQuerier]) error {
	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTxnOnce(ctx, db.writePool, db.factory, args)

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

func execTxnOnce(ctx context.Context, pool *pgxpool.Pool, factory QueryFactoryImpl, args TxArgs[ReadWriteQuerier]) (txnErr error) {
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

	txnErr = args.QueryFn(ctx, txn, factory.TxnQuerier(txn))
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
