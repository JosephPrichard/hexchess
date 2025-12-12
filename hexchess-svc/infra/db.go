package infra

import (
	"context"
	"hexchess-svc/db"
	"log/slog"
	"slices"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxFn[Ret any] func(query *db.Queries) (Ret, error)
type BeginTxFn = func(ctx context.Context) (pgx.Tx, error)

type Postgres struct {
	Query   *db.Queries
	Pool    *pgxpool.Pool
	noopTxn bool
}

func (db Postgres) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

type RedisAddrs struct {
	CacheAddr  string
	PubsubAddr string
}

type RedisNames struct {
	LeaderboardZSet string
	GamesZSet       string
	ActiveUsersZSet string
	GamesChan       string
	UsersChan       string
	GamesCountChan  string
	ActiveCountChan string
}

const (
	LeaderboardZSet = "leaderboard"
	GamesZSet       = "games"
	ActiveUsersZSet = "active_users"
	GamesChan       = "games_chan"
	UsersChan       = "users_chan"
	GamesCountChan  = "games_count"
	ActiveCountChan = "active_count"
)

var DefaultRedisNames = RedisNames{
	LeaderboardZSet: LeaderboardZSet,
	GamesZSet:       GamesZSet,
	ActiveUsersZSet: ActiveUsersZSet,
	GamesChan:       GamesChan,
	UsersChan:       UsersChan,
	GamesCountChan:  GamesCountChan,
	ActiveCountChan: ActiveCountChan,
}

type Redis struct {
	Cache  *redis.Pool
	PubSub *redis.Pool
	RedisAddrs
	RedisNames
}

func (rdb *Redis) Close() {
	if rdb.Cache != nil {
		rdb.Cache.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
}

type Databases struct {
	Pdb *Postgres
	Rdb *Redis
}

func (s Databases) Close() {
	s.Pdb.Close()
	s.Rdb.Close()
}

const MaxIdle = 3
const IdleTimeout = 240 * time.Second

func makeRedisDial(addr string) func() (redis.Conn, error) {
	return func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		return c, err
	}
}

func MakeRdb(addrs RedisAddrs, names RedisNames) *Redis {
	var pubsub *redis.Pool
	if addrs.PubsubAddr != "" {
		pubsub = &redis.Pool{
			MaxIdle:     MaxIdle,
			IdleTimeout: IdleTimeout,
			Dial:        makeRedisDial(addrs.PubsubAddr),
		}
	}
	return &Redis{
		Cache: &redis.Pool{
			MaxIdle:     MaxIdle,
			IdleTimeout: IdleTimeout,
			Dial:        makeRedisDial(addrs.CacheAddr),
		},
		PubSub:     pubsub,
		RedisAddrs: addrs,
		RedisNames: names,
	}
}

func MakePostgres(query *db.Queries, pool *pgxpool.Pool) *Postgres {
	return &Postgres{Query: query, Pool: pool}
}

func MakeFakePostgres(query *db.Queries) *Postgres {
	return &Postgres{Query: query, noopTxn: true}
}

type TxnArgs[Ret any] struct {
	Ctx          context.Context
	Postgres     *Postgres
	TxFn         TxFn[Ret]
	ErrAllowList []error // errors where we are allowed to commit instead of rollback
}

func WithTxn[Ret any](args TxnArgs[Ret]) (ret Ret, err error) {
	if args.Postgres.noopTxn {
		return args.TxFn(args.Postgres.Query)
	}

	ctx := args.Ctx
	tx, err := args.Postgres.Pool.Begin(args.Ctx)
	if err != nil {
		return ret, err
	}

	defer func() {
		if p := recover(); p != nil {
			// a panic occurred, rollback and repanic
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
			panic(p)
		}
		if err != nil && !slices.Contains(args.ErrAllowList, err) {
			// something went wrong, rollback
			if err := tx.Rollback(ctx); err != nil {
				slog.ErrorContext(ctx, "failed to rollback tx", "err", err)
			}
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				slog.ErrorContext(ctx, "failed to commit tx", "err", commitErr)
				err = commitErr // due to the error allowList, we may still want to commit with an error, so only assign when there is a commit err
			}
		}
	}()

	ret, err = args.TxFn(args.Postgres.Query.WithTx(tx))
	return
}
