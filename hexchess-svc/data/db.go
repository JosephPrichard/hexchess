package data

import (
	"context"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/db"
	"log/slog"
	"time"
)

type TxFn[Ret any] func(q *db.Queries) (Ret, error)
type BeginTxFn = func(ctx context.Context) (pgx.Tx, error)

type PgDB struct {
	Q         *db.Queries
	BeginTxFn BeginTxFn
	noopTxn   bool
}

type Rdb struct {
	*redis.Pool
	LeaderboardZSet string
	GamesZSet       string
	ActiveUsersZSet string
	GamesChan       string
	UsersChan       string
	GamesCountChan  string
	UsersCountChan  string
}

type Stores struct {
	PgDB
	Rdb Rdb
}

func MakeRdb(addr string) Rdb {
	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}
	return Rdb{
		Pool:            pool,
		LeaderboardZSet: LeaderboardZSet,
		GamesZSet:       GamesZSet,
		ActiveUsersZSet: ActiveUsersZSet,
		GamesChan:       "games",
		UsersChan:       "users",
		GamesCountChan:  "games_count",
		UsersCountChan:  "users_count",
	}
}

func MakeDbClient(q *db.Queries, pool *pgxpool.Pool) PgDB {
	return PgDB{Q: q, BeginTxFn: pool.Begin}
}

func MakeFakeDbClient(q *db.Queries) PgDB {
	return PgDB{Q: q, noopTxn: true}
}

func WithTxn[Ret any](ctx context.Context, db PgDB, txFn TxFn[Ret]) (ret Ret, err error) {
	if db.noopTxn {
		// a db client can be a fake when testing. if so, do not begin or commit a new Txn, since the test is already in a txn
		return txFn(db.Q)
	}

	tx, err := db.BeginTxFn(ctx)
	if err != nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			// a panic occurred, rollback and repanic
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
			panic(p)
		} else if err != nil {
			// something went wrong, rollback
			slog.ErrorContext(ctx, "failed to complete tx, rolling back", "err", err)
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
		} else {
			// all good, commit
			err = tx.Commit(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "failed to commit tx", "err", err)
			}
		}
	}()
	ret, err = txFn(db.Q.WithTx(tx))
	return
}
