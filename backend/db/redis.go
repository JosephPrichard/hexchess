package db

import (
	"context"
	"fmt"
	"hexchess-lib/config"
	"hexchess-lib/logutil"
	"log/slog"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/redis/go-redis/v9"
)

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
	Primary    redis.UniversalClient
	PubSub     *redigo.Pool
	PubsubAddr string `json:"pubsubAddr"`
	RedisNames
	primaryRefresher  *RedisTokenRefresher
	pubsubRefresher   *RedisTokenRefresher
}

func (rdb *Redis) Close() {
	if rdb.Primary != nil {
		rdb.Primary.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
	if rdb.primaryRefresher != nil {
		rdb.primaryRefresher.Shutdown()
	}
	if rdb.pubsubRefresher != nil {
		rdb.pubsubRefresher.Shutdown()
	}
}

type RedisConfig struct {
	SorAddr           []string       `json:"sorAddr"`
	SorClusterName    string         `json:"sorClusterName"`
	SorUsername       string         `json:"userName"`
	PubsubAddr        string         `json:"pubsubAddr"`
	PubsubClusterName string         `json:"pubsubClusterName"`
	PubsubUsername    string         `json:"pubsubUsername"`
	ActiveProfile     config.Profile `json:"activeProfile"`
	AWSServiceName    string         `json:"AWSServiceName"`
	AWSRegion         string         `json:"awsRegion"`
	Names             *RedisNames    `json:"names"`
}

func NewRedis(ctx context.Context, redisCfg RedisConfig) Redis {
	slog.Info("creating redis client", "config", redisCfg)

	if redisCfg.Names == nil {
		redisCfg.Names = &DefaultRedisNames
	}
	
	var primaryRefresher *RedisTokenRefresher
	var pubsubRefresher *RedisTokenRefresher

	redisClientOpts := &redis.UniversalOptions{
		Addrs:          redisCfg.SorAddr,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	}
	if redisCfg.ActiveProfile != config.Local {
		primaryRefresher = NewRedisTokenRefresher(ctx, redisCfg.AWSRegion, redisCfg.SorUsername, redisCfg.SorClusterName)
		redisClientOpts.CredentialsProvider = NewCredentialsProvider(primaryRefresher)
	}
	redisClient := redis.NewUniversalClient(redisClientOpts)

	var pubsubPool *redigo.Pool
	if redisCfg.PubsubAddr != "" {
		pubsubPool = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
		}
		if redisCfg.ActiveProfile != config.Local {
			pubsubRefresher = NewRedisTokenRefresher(ctx, redisCfg.AWSRegion, redisCfg.PubsubUsername, redisCfg.PubsubClusterName)
			pubsubPool.Dial = NewSecureDialer(pubsubRefresher, redisCfg.PubsubAddr)
		} else {
			pubsubPool.Dial = func() (redigo.Conn, error) {
				return redigo.Dial("tcp", redisCfg.PubsubAddr)
			}
		}
	}

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logutil.Fatal("execute redis startup cmd", err)
	}

	slog.Info("created redis client", "redisClientKind", fmt.Sprintf("%T", redisClient))

	return Redis{
		Primary:    redisClient,
		PubSub:     pubsubPool,
		RedisNames: *redisCfg.Names,
		PubsubAddr: redisCfg.PubsubAddr,
		primaryRefresher: primaryRefresher,
		pubsubRefresher: pubsubRefresher,
	}
}
