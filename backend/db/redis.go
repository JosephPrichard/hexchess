package db

import (
	"context"
	"fmt"
	"hexchess-svc/lib/logutil"
	"log/slog"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/redis/go-redis/v9"
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

type Redis struct {
	GameStore redis.UniversalClient
	Cache     redis.UniversalClient
	PubSub    *redigo.Pool
	RedisAddrs
	RedisNames
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

func makeRedigoPool(addr string) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     1,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			return redigo.Dial("tcp", addr)
		},
	}
}

type RedisCfg struct {
	Addrs       RedisAddrs `json:"addrs"`
	Profile     string     `json:"profile"`
	ServiceName string     `json:"serviceName"`
	ClusterName string     `json:"clusterName"`
	UserName    string     `json:"userName"`
	Region      string     `json:"region"`
	Names       *RedisNames
}

func NewRedis(ctx context.Context, redisCfg RedisCfg) (Redis, func()) {
	slog.Info("creating redis client", "cfg", redisCfg, "names", redisCfg.Names)

	if redisCfg.Names == nil {
		redisCfg.Names = &DefaultRedisNames
	}
	var psPool *redigo.Pool
	if redisCfg.Addrs.PubsubAddr != "" {
		psPool = makeRedigoPool(redisCfg.Addrs.PubsubAddr)
	}

	redisClientOpts := &redis.UniversalOptions{
		Addrs:          redisCfg.Addrs.SorAddr,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	}
	var connector *RedisConnector
	if redisCfg.Profile != "local" {
		connector = NewRedisConnector(ctx, redisCfg)
		redisClientOpts.CredentialsProvider = connector.CredentialsProvider
	}
	redisClient := redis.NewUniversalClient(redisClientOpts)

	if psPool != nil {
		conn, err := psPool.GetContext(ctx)
		if err != nil {
			logutil.Fatal("get redis conn", err)
		}
		defer conn.Close()
		if _, err := conn.Do("PING"); err != nil {
			logutil.Fatal("ping redis pubsub node", err)
		}
	}
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logutil.Fatal("ping redis sor node", err)
	}

	slog.Info("created redis client", "cfg", redisCfg, "names", redisCfg.Names, "redisClientKind", fmt.Sprintf("%T", redisClient))

	rdb := Redis{
		GameStore:  redisClient,
		Cache:      redisClient,
		PubSub:     psPool,
		RedisAddrs: redisCfg.Addrs,
		RedisNames: *redisCfg.Names,
	}
	closer := func() {
		rdb.Close()
		if connector != nil {
			connector.Stop()
		}
	}
	return rdb, closer
}
