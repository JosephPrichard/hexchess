package db

import (
	"context"
	"fmt"
	"hexchess-svc/lib/config"
	"hexchess-svc/lib/logutil"
	"log/slog"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/redis/go-redis/v9"
)

type RedisDSNs struct {
	SorAddr           []string `json:"sorAddr"`
	SorClusterName    string   `json:"sorClusterName"`
	SorUsername       string   `json:"userName"`
	PubsubAddr        string   `json:"pubsubAddr"`
	PubsubClusterName string   `json:"pubsubClusterName"`
	PubsubUsername    string   `json:"pubsubUsername"`
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
	Primary redis.UniversalClient
	PubSub  *redigo.Pool
	RedisDSNs
	RedisNames
}

func (rdb *Redis) Close() {
	if rdb.Primary != nil {
		rdb.Primary.Close()
	}
	if rdb.PubSub != nil {
		rdb.PubSub.Close()
	}
}

type RedisCfg struct {
	DSNs           RedisDSNs      `json:"dsns"`
	ActiveProfile  config.Profile `json:"activeProfile"`
	AWSServiceName string         `json:"AWSServiceName"`
	AWSRegion      string         `json:"awsRegion"`
	Names          *RedisNames
}

func NewRedis(ctx context.Context, redisCfg RedisCfg) Redis {
	slog.Info("creating redis client", "config", redisCfg)

	if redisCfg.Names == nil {
		redisCfg.Names = &DefaultRedisNames
	}

	redisClientOpts := &redis.UniversalOptions{
		Addrs:          redisCfg.DSNs.SorAddr,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	}
	if redisCfg.ActiveProfile != config.Local {
		refresherSOR := NewRedisTokenRefresher(ctx, redisCfg.AWSRegion, redisCfg.DSNs.SorUsername, redisCfg.DSNs.SorClusterName)
		redisClientOpts.CredentialsProvider = refresherSOR.CredentialsProviderFunc
	}
	redisClient := redis.NewUniversalClient(redisClientOpts)

	var pubsubPool *redigo.Pool
	if redisCfg.DSNs.PubsubAddr != "" {
		pubsubPool = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redigo.Conn, error) {
				return redigo.Dial("tcp", redisCfg.DSNs.PubsubAddr)
			},
		}
		if redisCfg.ActiveProfile != config.Local {
			refresherPubsub := NewRedisTokenRefresher(ctx, redisCfg.AWSRegion, redisCfg.DSNs.PubsubUsername, redisCfg.DSNs.PubsubClusterName)
			pubsubPool.Dial = refresherPubsub.DialFunc(redisCfg.DSNs.PubsubAddr)
		}
	}

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logutil.Fatal("execute redis startup cmd", err)
	}

	slog.Info("created redis client", "config", redisCfg, "redisClientKind", fmt.Sprintf("%T", redisClient))

	return Redis{
		Primary:    redisClient,
		PubSub:     pubsubPool,
		RedisDSNs:  redisCfg.DSNs,
		RedisNames: *redisCfg.Names,
	}
}
