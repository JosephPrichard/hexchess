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

type Pdb struct {
	Query *db.Queries   // always initialized, used by application code to talk with the db
	pool  *pgxpool.Pool // used for creating new txn whenever Pdb is not a txn
	txn   pgx.Tx        // a postgres struct can be a txn - this is used in testing where we want to fake an operation as a txn
}

func (pdb *Pdb) Close() {
	if pdb.pool != nil {
		pdb.pool.Close()
	}
}

func (pdb *Pdb) GetPool() *pgxpool.Pool {
	if pdb.pool == nil {
		panic("pgxpool not initialized")
	}
	return pdb.pool
}

func (pdb *Pdb) GetTxn() pgx.Tx {
	if pdb.txn == nil {
		panic("txn not initialized")
	}
	return pdb.txn
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
	Pdb *Pdb
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

func MakePostgres(query *db.Queries, pool *pgxpool.Pool) *Pdb {
	return &Pdb{Query: query, pool: pool}
}

func MakeTxnPostgres(txn pgx.Tx) *Pdb {
	return &Pdb{Query: db.New(txn), txn: txn}
}

type TxFn[Ret any] func(query *db.Queries) (Ret, error)
type BeginTxFn = func(ctx context.Context) (pgx.Tx, error)

type TxnArgs[Ret any] struct {
	Ctx          context.Context
	Pdb          *Pdb
	TxFn         TxFn[Ret]
	ErrAllowList []error // errors where we are allowed to commit instead of rollback
}

func WithTxn[Ret any](args TxnArgs[Ret]) (ret Ret, err error) {
	pdb := args.Pdb
	if pdb.txn != nil {
		return args.TxFn(db.New(pdb.txn))
	}

	ctx := args.Ctx
	tx, err := pdb.GetPool().Begin(args.Ctx)
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

	ret, err = args.TxFn(pdb.Query.WithTx(tx))
	return
}
