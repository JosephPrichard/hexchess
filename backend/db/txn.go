package db

import (
	"context"
	"errors"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"math"
	"math/rand"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type QueryFn func(ctx context.Context, querier sqlc.Querier) error

type TxArgs struct {
	QueryFn      QueryFn
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}

func (pdb *FakeDB) ExecTx(ctx context.Context, args TxArgs) (err error) {
	// a fake postgres instance is already running in a txn, noop the txn
	return args.QueryFn(ctx, sqlc.New(pdb.testingTxn))
}

func (pdb *ImplDB) ExecTx(ctx context.Context, args TxArgs) error {
	execTx := func(ctx context.Context, args TxArgs) (retErr error) {
		if args.Isolation == "" {
			args.Isolation = pgx.ReadCommitted
		}
		tx, retErr := pdb.pool.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: args.Isolation,
		})
		if retErr != nil {
			return
		}

		defer func() {
			if p := recover(); p != nil {
				if err := tx.Rollback(ctx); err != nil {
					slog.ErrorContext(ctx, "failed to rollback txn", "error", err)
				}
				panic(p)
			}
			isErrAllowListed := slices.Contains(args.ErrAllowlist, retErr)
			if retErr != nil && !isErrAllowListed {
				if dbErr := tx.Rollback(ctx); dbErr != nil {
					slog.ErrorContext(ctx, "failed to rollback txn", "error", dbErr)
					retErr = dbErr
				}
			} else {
				if dbErr := tx.Commit(ctx); dbErr != nil {
					slog.ErrorContext(ctx, "failed to commit txn", "error", dbErr)
					retErr = dbErr
				}
			}
		}()

		retErr = args.QueryFn(ctx, pdb.q.WithTx(tx))
		return
	}

	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTx(ctx, args)

		if isSerializationFailure(err) {
			slog.WarnContext(ctx, "retrying transaction", "error", err, "retry", i)

			time.Sleep(exponentialBackoff(i, 2, 50*time.Millisecond))
			continue
		}
		break
	}
	if isSerializationFailure(err) {
		slog.ErrorContext(ctx, "exhausted transaction retries", "error", err, "retryCount", args.RetryCount)
	}

	return err
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == ErrPgSerializationFailure || pgErr.Code == ErrPgDeadlock)
}

func exponentialBackoff(retry int, multiplier float64, base time.Duration) time.Duration {
	backoff := float64(base) * math.Pow(multiplier, float64(retry))
	jitter := rand.Float64() * float64(base)
	return time.Duration(backoff + jitter)
}
