package db

import (
	"context"
	"log/slog"
	"slices"
)

type TxFn[T any] func(ctx context.Context, query *Queries) (T, error)

func RunInTx[T any](ctx context.Context, pdb *Postgres, errAllowList []error, txFn TxFn[T]) (ret T, err error) {
	if pdb.testingTxn != nil {
		return txFn(ctx, New(pdb.testingTxn))
	}
	tx, err := pdb.Pool.Begin(ctx)
	if err != nil {
		return ret, err
	}

	defer func() {
		if p := recover(); p != nil {
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
			panic(p)
		} else if err != nil && !slices.Contains(errAllowList, err) {
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

	ret, err = txFn(ctx, pdb.Query.WithTx(tx))
	return
}
