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
	SorAddr           []string `json:"sorAddr"`
	SorClusterName    string   `json:"sorClusterName"`
	PubsubAddr        string   `json:"pubsubAddr"`
	PubsubClusterName string   `json:"pubsubClusterName"`
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

type RedisCfg struct {
	Addrs         RedisAddrs `json:"addrs"`
	ActiveProfile string     `json:"activeProfile"`
	ServiceName   string     `json:"serviceName"`
	UserName      string     `json:"userName"`
	Region        string     `json:"region"`
	Names         *RedisNames
}

func NewRedis(ctx context.Context, redisCfg RedisCfg) (Redis, func()) {
	slog.Info("creating redis client", "cfg", redisCfg, "names", redisCfg.Names)

	if redisCfg.Names == nil {
		redisCfg.Names = &DefaultRedisNames
	}

	var connectorSOR *RedisConnector

	redisClientOpts := &redis.UniversalOptions{
		Addrs:          redisCfg.Addrs.SorAddr,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	}
	if redisCfg.ActiveProfile != "local" {
		connectorSOR = NewRedisConnector(ctx, redisCfg.Region, redisCfg.UserName, redisCfg.Addrs.SorClusterName)
		redisClientOpts.CredentialsProvider = connectorSOR.CredentialsProvider
	}
	redisClient := redis.NewUniversalClient(redisClientOpts)

	var pubsubPool *redigo.Pool
	var connectorPubsub *RedisConnector

	if redisCfg.Addrs.PubsubAddr != "" {
		pubsubPool = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redigo.Conn, error) {
				return redigo.Dial("tcp", redisCfg.Addrs.PubsubAddr)
			},
		}
		if redisCfg.ActiveProfile != "local" {
			connectorPubsub = NewRedisConnector(ctx, redisCfg.Region, redisCfg.UserName, redisCfg.Addrs.SorClusterName)
			pubsubPool.Dial = func() (redigo.Conn, error) {
				username, password := connectorPubsub.CredentialsProvider()
				return redigo.Dial("tcp", redisCfg.Addrs.PubsubAddr, redigo.DialUsername(username), redigo.DialPassword(password))
			}
		}
	}

	if pubsubPool != nil {
		conn, err := pubsubPool.GetContext(ctx)
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
		PubSub:     pubsubPool,
		RedisAddrs: redisCfg.Addrs,
		RedisNames: *redisCfg.Names,
	}
	closer := func() {
		rdb.Close()
		if connectorSOR != nil {
			connectorSOR.Stop()
		}
		if connectorPubsub != nil {
			connectorPubsub.Stop()
		}
	}
	return rdb, closer
}
