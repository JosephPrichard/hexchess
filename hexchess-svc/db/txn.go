package db

import (
	"context"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"slices"
)

type TxnArgs struct {
	QueryFn      func(ctx context.Context, query *sqlc.Queries) error
	ErrAllowlist []error
}

func (pdb *PostgresDB) ExecTx(ctx context.Context, args TxnArgs) (err error) {
	tx, err := pdb.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
			panic(p)
		} else if err != nil && !slices.Contains(args.ErrAllowlist, err) {
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
	return
}

func (pdb *FakeDB) ExecTx(ctx context.Context, args TxnArgs) (err error) {
	// a fake postgres instance is already running in a txn, so noop the txn
	return args.QueryFn(ctx, sqlc.New(pdb.testingTxn))
}
