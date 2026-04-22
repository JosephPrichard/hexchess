package db

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"math"
	"math/rand"
	"slices"
	"time"
)

type QueryFn func(ctx context.Context, querier sqlc.Querier) error

type TxArgs struct {
	QueryFn      QueryFn
	ErrAllowlist []error
	Isolation    pgx.TxIsoLevel
	RetryCount   int
}

func (pdb *PostgresDB) ExecTx(ctx context.Context, args TxArgs) error {
	execTx := func(ctx context.Context, args TxArgs) error {
		if args.Isolation == "" {
			args.Isolation = pgx.ReadCommitted
		}
		tx, err := pdb.pool.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: args.Isolation,
		})
		if err != nil {
			return err
		}

		defer func() {
			if p := recover(); p != nil {
				if err := tx.Rollback(ctx); err != nil {
					slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
				}
				panic(p)
			}
			isErrAllowListed := slices.Contains(args.ErrAllowlist, err)
			if err != nil && !isErrAllowListed {
				if dbErr := tx.Rollback(ctx); dbErr != nil {
					slog.ErrorContext(ctx, "failed to rollback tx", "err", dbErr)
					err = dbErr
				}
			} else {
				if dbErr := tx.Commit(ctx); dbErr != nil {
					slog.ErrorContext(ctx, "failed to commit tx", "err", dbErr)
					err = dbErr
				}
			}
		}()

		err = args.QueryFn(ctx, pdb.q.WithTx(tx))
		return err
	}

	if args.RetryCount == 0 {
		args.RetryCount = 1
	}

	var err error
	for i := range args.RetryCount {
		err = execTx(ctx, args)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == ErrPgSerializationFailure || pgErr.Code == ErrPgDeadlock) {
			time.Sleep(exponentialBackoff(i, 2, 50*time.Millisecond))
			continue
		}
		break
	}
	return err
}

func exponentialBackoff(retry int, multiplier float64, base time.Duration) time.Duration {
	backoff := float64(base) * math.Pow(multiplier, float64(retry))
	jitter := rand.Float64() * float64(base)
	return time.Duration(backoff + jitter)
}

func (pdb *FakeDB) ExecTx(ctx context.Context, args TxArgs) (err error) {
	// a fake postgres instance is already running in a txn, noop the txn
	return args.QueryFn(ctx, sqlc.New(pdb.testingTxn))
}
