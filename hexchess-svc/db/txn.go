package db

import (
	"context"
	"log/slog"
	"slices"
)

type TxFn func(ctx context.Context, query *Queries) error

func (pdb *PostgreSQL) RunInTx(ctx context.Context, errAllowList []error, txFn TxFn) (err error) {
	if pdb.testingTxn != nil {
		return txFn(ctx, New(pdb.testingTxn))
	}
	tx, err := pdb.GetPool().Begin(ctx)
	if err != nil {
		return err
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

	err = txFn(ctx, pdb.Query.WithTx(tx))
	return
}
