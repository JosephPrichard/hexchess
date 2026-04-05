package db

import (
	"context"
	"fmt"
	"hexchess-svc/db/sqlc"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type DB interface {
	Querier() sqlc.Querier
	ExecTx(context.Context, Tx) error
	Close()
}

type PostgresDB struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func (pdb *PostgresDB) Querier() sqlc.Querier {
	return pdb.q
}

func (pdb *PostgresDB) Close() {
	pdb.pool.Close()
}

type FakeDB struct {
	testingTxn pgx.Tx
}

func (pdb *FakeDB) Querier() sqlc.Querier {
	return sqlc.New(pdb.testingTxn)
}

func (pdb *FakeDB) Close() {
	if err := pdb.testingTxn.Rollback(context.Background()); err != nil {
		panic(fmt.Sprintf("failed to rollback testing txn: %v", err))
	}
}

func MakeDB(pool *pgxpool.Pool) DB {
	return &PostgresDB{q: sqlc.New(pool), pool: pool}
}

func MakeFakeDB(txn pgx.Tx) DB {
	return &FakeDB{testingTxn: txn}
}

type RedisAddrs struct {
	GameStoreAddr string `json:"gameStoreAddr"`
	CacheAddr     string `json:"cacheAddr"`
	PubsubAddr    string `json:"pubsubAddr"`
}

type RedisNames struct {
	LeaderboardZSet         string `json:"leaderboardZSet"`
	GamesZSet               string `json:"gamesZSet"`
	ActiveUsersZSet         string `json:"activeUsersZSet"`
	GameChatsZSet           string `json:"gameChatsZSet"`
	GamesChannel            string `json:"gamesChannel"`
	UsersChannel            string `json:"usersChannel"`
	GamesCountChannel       string `json:"gamesCountChannel"`
	ActiveCountChannel      string `json:"activeCountChannel"`
	FinishGameStreamKey     string `json:"finishGameStreamKey"`
	FinishGameConsumerGroup string `json:"finishGameConsumerGroup"`
}

const (
	LeaderboardZSet     = "leaderboard"
	GamesZSet           = "games"
	ActiveUsersZSet     = "active_users"
	GameChatsZSet       = "chats"
	GamesChannel        = "games_channel"
	UsersChannel        = "users_channel"
	GamesCountChannel   = "games_count_channel"
	ActiveCountChannel  = "active_count_channel"
	FinishGameStreamKey = "finish_game_events"
)

var DefaultRedisNames = RedisNames{
	LeaderboardZSet:     LeaderboardZSet,
	GamesZSet:           GamesZSet,
	ActiveUsersZSet:     ActiveUsersZSet,
	GameChatsZSet:       GameChatsZSet,
	GamesChannel:        GamesChannel,
	UsersChannel:        UsersChannel,
	GamesCountChannel:   GamesCountChannel,
	ActiveCountChannel:  ActiveCountChannel,
	FinishGameStreamKey: FinishGameStreamKey,
}

type Redis struct {
	GameStore *redis.Client
	Cache     *redis.Client
	PubSub    *redigo.Pool
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

func MakeRdb(addrs RedisAddrs, names *RedisNames) Redis {
	if names == nil {
		names = &DefaultRedisNames
	}
	var ps *redigo.Pool
	if addrs.PubsubAddr != "" {
		ps = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redigo.Conn, error) {
				return redigo.Dial("tcp", addrs.PubsubAddr)
			},
		}
	}
	return Redis{
		GameStore: redis.NewClient(&redis.Options{
			Addr: addrs.GameStoreAddr,
		}),
		Cache: redis.NewClient(&redis.Options{
			Addr: addrs.CacheAddr,
		}),
		PubSub:     ps,
		RedisAddrs: addrs,
		RedisNames: *names,
	}
}
