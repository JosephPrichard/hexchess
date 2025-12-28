package db

import (
	"context"
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
	Query      *Queries
	Pool       *pgxpool.Pool
	testingTxn pgx.Tx
}

func (pdb *PostgreSQL) Close() {
	if pdb.Pool != nil {
		pdb.Pool.Close()
	}
	if pdb.testingTxn != nil {
		if err := pdb.testingTxn.Rollback(context.Background()); err != nil {
			panic(fmt.Sprintf("failed to rollback testing txn: %v", err))
		}
	}
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
	if s.Pdb != nil {
		s.Pdb.Close()
	}
	if s.Rdb != nil {
		s.Rdb.Close()
	}
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

func MakePostgres(pool *pgxpool.Pool) *PostgreSQL {
	return &PostgreSQL{Query: New(pool), Pool: pool}
}

func MakeTestTxnPostgres(txn pgx.Tx) *PostgreSQL {
	return &PostgreSQL{Query: New(txn), testingTxn: txn}
}
