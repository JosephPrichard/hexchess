package svc

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/db"
	"log/slog"
)

type DB struct {
	Q    *db.Queries
	pool *pgxpool.Pool
}

type Databases struct {
	Rdb  *redis.Client
	PgDB DB
}

func MakeDbClient(q *db.Queries, pool *pgxpool.Pool) DB {
	return DB{Q: q, pool: pool}
}

type TxFn[Ret any] func(q *db.Queries) (Ret, error)
type BeginTxFn = func(ctx context.Context) (pgx.Tx, error)

func WithTransaction[Ret any](ctx context.Context, pgDB DB, txFn TxFn[Ret]) (ret Ret, err error) {
	trace := ctx.Value(TraceKey)
	tx, err := pgDB.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			// a panic occurred, rollback and repanic
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			// something went wrong, rollback
			slog.Error("failed to complete tx, rolling back", "err", err, "trace", trace)
			_ = tx.Rollback(ctx)
		} else {
			// all good, commit
			err = tx.Commit(ctx)
			if err != nil {
				slog.Error("failed to commit tx", "err", err, "trace", trace)
			}
		}
	}()
	ret, err = txFn(pgDB.Q.WithTx(tx))
	return
}
