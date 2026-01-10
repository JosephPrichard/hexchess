package db

import (
	"context"
	"errors"
	"fmt"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisAddrs struct {
	CacheAddr  string
	PubsubAddr string
}

type RedisNames struct {
	LeaderboardZSet  string
	GamesZSet        string
	ActiveUsersZSet  string
	GameChatsPostfix string
	GamesChan        string
	UsersChan        string
	GamesCountChan   string
	ActiveCountChan  string
}

const (
	LeaderboardZSet = "leaderboard"
	GamesZSet       = "games"
	ActiveUsersZSet = "active_users"
	GameChatsPrefix = "chats"
	GamesChan       = "games_chan"
	UsersChan       = "users_chan"
	GamesCountChan  = "games_count"
	ActiveCountChan = "active_count"
)

var DefaultRedisNames = RedisNames{
	LeaderboardZSet:  LeaderboardZSet,
	GamesZSet:        GamesZSet,
	ActiveUsersZSet:  ActiveUsersZSet,
	GameChatsPostfix: GameChatsPrefix,
	GamesChan:        GamesChan,
	UsersChan:        UsersChan,
	GamesCountChan:   GamesCountChan,
	ActiveCountChan:  ActiveCountChan,
}

type Postgres interface {
	Query() *Queries
	HealthCheck(context.Context) error
	RunInTx(context.Context, TxnArgs) error
	Close()
}

type PostgresDB struct {
	q    *Queries
	pool *pgxpool.Pool
}

func (pdb *PostgresDB) HealthCheck(ctx context.Context) error {
	_, err := pdb.pool.Exec(ctx, "SELECT 1;")
	return err
}

func (pdb *PostgresDB) Query() *Queries {
	return pdb.q
}

func (pdb *PostgresDB) Close() {
	pdb.pool.Close()
}

type PostgresFake struct {
	testingTxn pgx.Tx
}

func (pdb *PostgresFake) HealthCheck(_ context.Context) error {
	return errors.New("health check failed for fake postgres impl")
}

func (pdb *PostgresFake) Query() *Queries {
	return New(pdb.testingTxn)
}

func (pdb *PostgresFake) Close() {
	if err := pdb.testingTxn.Rollback(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to rollback testing txn: %v", err))
	}
}

func MakePostgres(pool *pgxpool.Pool) Postgres {
	return &PostgresDB{q: New(pool), pool: pool}
}

func MakeFakePostgres(txn pgx.Tx) Postgres {
	return &PostgresFake{testingTxn: txn}
}

type Redis struct {
	Cache  *redis.Client
	PubSub *redigo.Pool
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

func MakeRdb(addrs RedisAddrs, names RedisNames) Redis {
	var ps *redigo.Pool
	if addrs.PubsubAddr != "" {
		ps = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redigo.Conn, error) {
				c, err := redigo.Dial("tcp", addrs.PubsubAddr)
				if err != nil {
					return nil, err
				}
				return c, err
			},
		}
	}
	return Redis{
		Cache: redis.NewClient(&redis.Options{
			Addr: addrs.CacheAddr,
		}),
		PubSub:     ps,
		RedisAddrs: addrs,
		RedisNames: names,
	}
}
