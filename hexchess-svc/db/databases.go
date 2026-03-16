package db

import (
	"context"
	"fmt"
	"hexchess-svc/db/sqlc"
	"strconv"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type DB interface {
	Queries() *sqlc.Queries
	ExecTx(context.Context, TxnArgs) error
	Close()
}

type PostgresDB struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func (pdb *PostgresDB) Queries() *sqlc.Queries {
	return pdb.q
}

func (pdb *PostgresDB) Close() {
	pdb.pool.Close()
}

type FakeDB struct {
	testingTxn pgx.Tx
}

func (pdb *FakeDB) Queries() *sqlc.Queries {
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
	CacheAddr  string
	PubsubAddr string
}

type RedisNames struct {
	LeaderboardZSet         string
	GamesZSet               string
	ActiveUsersZSet         string
	GameChatsZSet           string
	GamesChannel            string
	UsersChannel            string
	GamesCountChannel       string
	ActiveCountChannel      string
	FinishGameStreamKey     string
	FinishGameConsumerGroup string
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

func (names *RedisNames) GetLeaderboardZSet(mode string) string {
	return names.LeaderboardZSet + "/mode/" + mode
}

func (names *RedisNames) GetUserGameZSet(id int64) string {
	return names.GamesZSet + "/user/" + strconv.Itoa(int(id))
}

func (names *RedisNames) MakeGameKey(gameID string) string {
	return "game/" + gameID
}

func (names *RedisNames) GetGameChatsZSet(gameKey string) string {
	return names.GameChatsZSet + "/" + gameKey
}

type Redis struct {
	Cache  *redis.Client
	PubSub *redigo.Pool
	Queue  *redis.Client
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
				return redigo.Dial("tcp", addrs.PubsubAddr)
			},
		}
	}
	return Redis{
		Cache: redis.NewClient(&redis.Options{
			Addr: addrs.CacheAddr,
		}),
		Queue: redis.NewClient(&redis.Options{
			Addr: addrs.CacheAddr,
		}),
		PubSub:     ps,
		RedisAddrs: addrs,
		RedisNames: names,
	}
}
