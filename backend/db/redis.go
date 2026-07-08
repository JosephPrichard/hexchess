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
	Consumer   redis.UniversalClient
	PubSub     *redigo.Pool
	PubsubAddr string `json:"pubsubAddr"`
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

type RedisConfig struct {
	// (required) URL(s) to connect to Primary cluter and Pubsub cluster. Primary must contain each node in the cluster. Pubsub is one node in the cluster.
	PrimaryAddr []string `json:"primaryAddr"`
	PubsubAddr  string   `json:"pubsubAddr"`

	// (optional) since consumers block an entire connection while reading
	// we need a seperate pool with the pool size set to the expected number of consumers = (streams * partitions_per_steam)
	// defaults to the redis connection pool default which is not suitable for the game events usecase
	ConsumerPoolSize int `json:"consumerPoolSize"`

	// (optional) username and password authentication is used for non-local setups
	// password authentication is an additional security layer, we're we really rely on network ACLs and firewalls to make redis access secure
	PrimaryUsername string `json:"primaryUsername"`
	PrimaryPassword string `json:"primaryPassword"`
	PubsubUsername  string `json:"pubsubUsername"`
	PubsubPassword  string `json:"pubsubPassword"`

	// (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	ActiveProfile config.Profile `json:"activeProfile"`

	// (optional) name data for key prefixes, zsets, etc. keep off in prod, swap out in integration tests
	Names *RedisNames `json:"names"`
}

func NewRedis(ctx context.Context, redisCfg RedisConfig) Redis {
	slog.Info("creating redis client", "config", redisCfg)

	if redisCfg.Names == nil {
		redisCfg.Names = &DefaultRedisNames
	}

	var primaryCredsProvider func() (string, string)

	if redisCfg.ActiveProfile != config.Local {
		primaryCredsProvider = func() (string, string) {
			return redisCfg.PrimaryUsername, redisCfg.PrimaryPassword
		}
	}

	var pubsubDialer func() (redigo.Conn, error)

	if redisCfg.ActiveProfile != config.Local {
		pubsubDialer = func() (redigo.Conn, error) {
			return redigo.Dial("tcp", redisCfg.PubsubAddr, redigo.DialUsername(redisCfg.PubsubUsername), redigo.DialPassword(redisCfg.PubsubPassword))
		}
	} else {
		pubsubDialer = func() (redigo.Conn, error) {
			return redigo.Dial("tcp", redisCfg.PubsubAddr)
		}
	}

	primaryRedisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:               redisCfg.PrimaryAddr,
		DialTimeout:         3 * time.Second,
		ReadTimeout:         3 * time.Second,
		WriteTimeout:        3 * time.Second,
		MaxRedirects:        10,
		CredentialsProvider: primaryCredsProvider,
	})

	consumerRedisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:               redisCfg.PrimaryAddr,
		PoolSize:            redisCfg.ConsumerPoolSize,
		DialTimeout:         3 * time.Second,
		ReadTimeout:         3 * time.Second,
		WriteTimeout:        3 * time.Second,
		MaxRedirects:        10,
		CredentialsProvider: primaryCredsProvider,
	})

	var pubsubPool *redigo.Pool
	if redisCfg.PubsubAddr != "" {
		pubsubPool = &redigo.Pool{
			MaxIdle:     1,
			IdleTimeout: 240 * time.Second,
			Dial:        pubsubDialer,
		}
	}

	if err := primaryRedisClient.Ping(ctx).Err(); err != nil {
		logutil.Fatal("execute redis startup cmd", err)
	}

	slog.Info("created redis client", "redisClientKind", fmt.Sprintf("%T", primaryRedisClient))

	return Redis{
		Primary:    primaryRedisClient,
		Consumer:   consumerRedisClient,
		PubSub:     pubsubPool,
		RedisNames: *redisCfg.Names,
		PubsubAddr: redisCfg.PubsubAddr,
	}
}
