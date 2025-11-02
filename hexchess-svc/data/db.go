package data

import (
	"context"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/db"
	"log/slog"
	"slices"
	"time"
)

type TxFn[Ret any] func(q *db.Queries) (Ret, error)
type BeginTxFn = func(ctx context.Context) (pgx.Tx, error)

type PgDB struct {
	Q       *db.Queries
	Pool    *pgxpool.Pool
	noopTxn bool
}

func (db PgDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

type Redis struct {
	Primary         *redis.Pool
	PubSub          *redis.Pool
	PrimaryAddr     string
	PubsubAddr      string
	LeaderboardZSet string
	GamesZSet       string
	ActiveUsersZSet string
	GamesChan       string
	UsersChan       string
	GamesCountChan  string
	ActiveCountChan string
}

func (rdb Redis) Close() {
	if rdb.Primary != nil {
		rdb.Primary.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
}

type Stores struct {
	PgDB
	Rdb Redis
}

func (s Stores) Close() {
	s.PgDB.Close()
	s.Rdb.Close()
}

const MaxIdle = 3
const IdleTimeout = 240 * time.Second

func MakeRdb(primaryAddr string, pubsubAddr string) Redis {
	makeDial := func(addr string) func() (redis.Conn, error) {
		return func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			return c, err
		}
	}
	var pubsub *redis.Pool
	if pubsubAddr != "" {
		pubsub = &redis.Pool{
			MaxIdle:     MaxIdle,
			IdleTimeout: IdleTimeout,
			Dial:        makeDial(pubsubAddr),
		}
	}
	return Redis{
		Primary: &redis.Pool{
			MaxIdle:     MaxIdle,
			IdleTimeout: IdleTimeout,
			Dial:        makeDial(primaryAddr),
		},
		PubSub:          pubsub,
		PrimaryAddr:     primaryAddr,
		PubsubAddr:      pubsubAddr,
		LeaderboardZSet: LeaderboardZSet,
		GamesZSet:       GamesZSet,
		ActiveUsersZSet: ActiveUsersZSet,
		GamesChan:       GamesChan,
		UsersChan:       UsersChan,
		GamesCountChan:  GamesCountChan,
		ActiveCountChan: ActiveCountChan,
	}
}

func MakeDbClient(q *db.Queries, pool *pgxpool.Pool) PgDB {
	return PgDB{Q: q, Pool: pool}
}

func MakeFakeDbClient(q *db.Queries) PgDB {
	return PgDB{Q: q, noopTxn: true}
}

type TxnArgs[Ret any] struct {
	Ctx          context.Context
	PgDB         PgDB
	TxFn         TxFn[Ret]
	ErrWhiteList []error // errors where we are allowed to commit instead of rollback
}

func WithTxn[Ret any](args TxnArgs[Ret]) (ret Ret, err error) {
	if args.PgDB.noopTxn {
		// a db client can be a fake when testing. if so, do not begin or commit a new Txn, since the test is already in a txn
		return args.TxFn(args.PgDB.Q)
	}

	ctx := args.Ctx
	tx, err := args.PgDB.Pool.Begin(args.Ctx)
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
		} else if err != nil && !slices.Contains(args.ErrWhiteList, err) {
			// something went wrong, rollback
			slog.ErrorContext(ctx, "failed to complete tx, rolling back", "err", err)
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
		} else {
			// all good, commit
			if cmtErr := tx.Commit(ctx); cmtErr != nil {
				slog.ErrorContext(ctx, "failed to commit tx", "err", err)
				err = cmtErr // due to the error allowlist, we may still want to commit with an error, so only assign when there is a commit err
			}
		}
	}()
	ret, err = args.TxFn(args.PgDB.Q.WithTx(tx))
	return
}
