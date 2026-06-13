package db

import (
	redigo "github.com/gomodule/redigo/redis"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/lib/testutil"
	"log/slog"
	"time"
)

type RedisAddrs struct {
	SorAddr    []string `json:"sorAddr"`
	PubsubAddr string   `json:"pubsubAddr"`
}

type RedisNames struct {
	LeaderboardZSet           string `json:"leaderboardZSet"`
	GamesZSet                 string `json:"gamesZSet"`
	ActiveUsersZSet           string `json:"activeUsersZSet"`
	GameChatsZSet             string `json:"gameChatsZSet"`
	GamesChannel              string `json:"gamesChannel"`
	TournamentsChannel        string `json:"tournamentsChannel"`
	UsersChannel              string `json:"usersChannel"`
	GamesCountChannel         string `json:"gamesCountChannel"`
	ActiveCountChannel        string `json:"activeCountChannel"`
	FinishGameStreamKey       string `json:"finishGameStreamKey"`
	FinishGameConsumerGroup   string `json:"finishGameConsumerGroup"`
	UpdtGameMetaStreamKey     string `json:"updtGameMetaStreamKey"`
	UpdtGameMetaConsumerGroup string `json:"updtGameMetaConsumerGroup"`
}

var DefaultRedisNames = RedisNames{
	LeaderboardZSet:           "leaderboard",
	GamesZSet:                 "games",
	ActiveUsersZSet:           "active_users",
	GameChatsZSet:             "chats",
	GamesChannel:              "games_channel",
	UsersChannel:              "users_channel",
	TournamentsChannel:        "tournaments_channel",
	GamesCountChannel:         "games_count_channel",
	ActiveCountChannel:        "active_count_channel",
	FinishGameStreamKey:       "finish_game_events",
	FinishGameConsumerGroup:   "finish_game_consumer_group",
	UpdtGameMetaStreamKey:     "updt_game_meta_events",
	UpdtGameMetaConsumerGroup: "updt_game_meta_consumer_group",
}

func MakeTestRedisNames() *RedisNames {
	return testutil.MakeTestNames(DefaultRedisNames)
}

type Redis struct {
	GameStore redis.UniversalClient
	Cache     redis.UniversalClient
	PubSub    *redigo.Pool
	RedisAddrs
	RedisNames
}

func makeRedigoPool(addr string, name string) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     1,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			slog.Info("dialing redis pubsub server", "name", name, "addr", addr)
			return redigo.Dial("tcp", addr)
		},
	}
}

func MakeRedis(addrs RedisAddrs, names *RedisNames) Redis {
	if names == nil {
		names = &DefaultRedisNames
	}
	var psPool *redigo.Pool
	if addrs.PubsubAddr != "" {
		psPool = makeRedigoPool(addrs.PubsubAddr, "pubsub")
	}
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:          addrs.SorAddr,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	})

	return Redis{
		// as of now, game store and cache are pointed to the same cluster.
		GameStore:  redisClient,
		Cache:      redisClient,
		PubSub:     psPool,
		RedisAddrs: addrs,
		RedisNames: *names,
	}
}

func (rdb *Redis) Close() {
	if rdb.GameStore != nil {
		rdb.GameStore.Close()
	}
	if rdb.Cache != nil {
		rdb.Cache.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
}
