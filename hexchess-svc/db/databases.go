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

type Postgres struct {
	Query      *Queries
	Pool       *pgxpool.Pool
	testingTxn pgx.Tx
}

func (pdb *Postgres) Close() {
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

func MakePostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{Query: New(pool), Pool: pool}
}
