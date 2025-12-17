package db

import (
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

type PostgreSQL struct {
	Query      *Queries      // always initialized, used by application code to talk with the db
	pool       *pgxpool.Pool // used for creating new txn whenever PostgreSQL is not a testingTxn
	testingTxn pgx.Tx        // a postgres struct can be a testingTxn - this is used in testing where we want to fake an operation as a txm
}

func (pdb *PostgreSQL) Close() {
	if pdb.pool != nil {
		pdb.pool.Close()
	}
}

func (pdb *PostgreSQL) GetPool() *pgxpool.Pool {
	if pdb.pool == nil {
		panic("pgxpool not initialized")
	}
	return pdb.pool
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

type Databases struct {
	Pdb *PostgreSQL
	Rdb *Redis
}

func (s Databases) Close() {
	s.Pdb.Close()
	s.Rdb.Close()
}

func MakeRdb(addrs RedisAddrs, names RedisNames) *Redis {
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
	return &Redis{
		Cache: redis.NewClient(&redis.Options{
			Addr: addrs.CacheAddr,
		}),
		PubSub:     ps,
		RedisAddrs: addrs,
		RedisNames: names,
	}
}

func MakePostgres(query *Queries, pool *pgxpool.Pool) *PostgreSQL {
	return &PostgreSQL{Query: query, pool: pool}
}

func MakeTestTxnPostgres(txn pgx.Tx) *PostgreSQL {
	return &PostgreSQL{Query: New(txn), testingTxn: txn}
}
