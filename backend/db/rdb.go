package db

import (
	redigo "github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/internal/logutil"
	"reflect"
	"time"
)

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
	TournamentsChannel      string `json:"tournamentsChannel"`
	UsersChannel            string `json:"usersChannel"`
	GamesCountChannel       string `json:"gamesCountChannel"`
	ActiveCountChannel      string `json:"activeCountChannel"`
	FinishGameStreamKey     string `json:"finishGameStreamKey"`
	FinishGameConsumerGroup string `json:"finishGameConsumerGroup"`
}

var DefaultRedisNames = RedisNames{
	LeaderboardZSet:     "leaderboard",
	GamesZSet:           "games",
	ActiveUsersZSet:     "active_users",
	GameChatsZSet:       "chats",
	GamesChannel:        "games_channel",
	UsersChannel:        "users_channel",
	TournamentsChannel:  "tournaments_channel",
	GamesCountChannel:   "games_count_channel",
	ActiveCountChannel:  "active_count_channel",
	FinishGameStreamKey: "finish_game_events",
}

func MakeTestRedisNames() *RedisNames {
	redisNames := DefaultRedisNames
	redisNamesPtr := &redisNames

	reflectRedisNames := reflect.ValueOf(redisNamesPtr)
	if reflectRedisNames.Kind() == reflect.Ptr {
		reflectRedisNames = reflectRedisNames.Elem()
	}
	if reflectRedisNames.Kind() != reflect.Struct {
		logutil.Fatal("reflectRedisNames is not a struct")
	}

	for i := range reflectRedisNames.NumField() {
		field := reflectRedisNames.Field(i)

		if field.Kind() == reflect.String && field.CanSet() {
			newValue := field.String() + "_" + uuid.New().String()
			field.SetString(newValue)
		}
	}
	return redisNamesPtr
}

type Redis struct {
	GameStore *redis.Client
	Cache     *redis.Client
	PubSub    *redigo.Pool
	RedisAddrs
	RedisNames
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

func (rdb *Redis) Close() {
	if rdb.Cache != nil {
		rdb.Cache.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
}
