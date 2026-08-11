package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/config"
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
	GameTimersZSet            string `json:"gameTimersZSet"`
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

var Constants = RedisNames{
	LeaderboardZSet:           "leaderboard",
	GamesZSet:                 "games",
	ActiveUsersZSet:           "active_users",
	GameChatsZSet:             "chats",
	GameTimersZSet:            "game_timers",
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
	PrimaryClient  redis.UniversalClient
	ConsumerClient redis.UniversalClient
	PubSubClient   *redigo.Pool
	PubsubAddr     string `json:"pubsubAddr"`
}

func (redis *Redis) PrimaryHealthCheck(ctx context.Context) error {
	err := redis.PrimaryClient.Ping(ctx).Err()
	if err != nil {
		slog.ErrorContext(ctx, "healthcheck error", "error", err, "kind", "redis")
	}
	return err
}

func (redis *Redis) PubsubHealthCheck(ctx context.Context) error {
	if redis.PubSubClient == nil {
		return nil
	}
	conn := redis.PubSubClient.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	if err != nil {
		slog.ErrorContext(ctx, "healthcheck error", "error", err, "kind", "redis")
	}
	return err
}

func (redis *Redis) Close() {
	if redis.PrimaryClient != nil {
		redis.PrimaryClient.Close()
	}
	if redis.PubSubClient != nil {
		redis.PubSubClient.Close()
	}
}

type RedisConfig struct {
	// (required) URL(s) to connect to Primary cluster and Pubsub cluster. Primary must contain each node in the cluster. Pubsub is one node in the cluster.
	PrimaryAddr []string `json:"primaryAddr"`
	PubsubAddr  string   `json:"pubsubAddr"`

	// (opt) since consumers block an entire connection while reading
	// we need a separate pool with the pool size set to the expected number of consumers = (streams * partitions_per_steam)
	// defaults to the redis connection pool default which is not suitable for the game events usecase
	ConsumerPoolSize int `json:"consumerPoolSize"`

	// (opt) username and password authentication is used for non-local setups
	// password authentication is an additional security layer,  we really rely on network ACLs and firewalls to make redis access secure
	PrimaryUsername string `json:"primaryUsername"`
	PrimaryPassword string `json:"primaryPassword"`
	PubsubUsername  string `json:"pubsubUsername"`
	PubsubPassword  string `json:"pubsubPassword"`

	// (required) profile for application is used to turn AWS authentication on (test/prod) and off (local)
	ActiveProfile config.Profile `json:"activeProfile"`
}

func NewRedis(ctx context.Context, redisCfg RedisConfig) Redis {
	slog.Info("creating redis clients", "config", redisCfg)

	var pubsubDialer func() (redigo.Conn, error)

	if redisCfg.ActiveProfile != config.Local {
		pubsubDialer = func() (redigo.Conn, error) {
			return redigo.Dial("tcp",
				redisCfg.PubsubAddr,
				redigo.DialUsername(redisCfg.PubsubUsername),
				redigo.DialPassword(redisCfg.PubsubPassword))
		}
	} else {
		pubsubDialer = func() (redigo.Conn, error) {
			return redigo.Dial("tcp", redisCfg.PubsubAddr)
		}
	}

	var pubsubPool *redigo.Pool
	if redisCfg.PubsubAddr != "" {
		pubsubPool = &redigo.Pool{
			MaxIdle:     8,
			IdleTimeout: 240 * time.Second,
			Dial:        pubsubDialer,
		}
	}

	// note: disabled until redis cluster issue is resolved
	var primaryCredsProvider func() (string, string)

	if redisCfg.ActiveProfile != config.Local {
		primaryCredsProvider = func() (string, string) {
			return redisCfg.PrimaryUsername, redisCfg.PrimaryPassword
		}
	}

	var consumerRedisClient redis.UniversalClient

	redisClientClient := NewUniversalClient(RedisClusterConfig{
		ActiveProfile:       redisCfg.ActiveProfile,
		Nodes:               redisCfg.PrimaryAddr,
		PoolSize:            0,
		CredentialsProvider: primaryCredsProvider,
	})
	if redisCfg.ConsumerPoolSize > 0 {
		consumerRedisClient = NewUniversalClient(RedisClusterConfig{
			ActiveProfile:       redisCfg.ActiveProfile,
			Nodes:               redisCfg.PrimaryAddr,
			PoolSize:            redisCfg.ConsumerPoolSize,
			CredentialsProvider: primaryCredsProvider,
		})
	}

	redisCache := Redis{
		PrimaryClient:  redisClientClient,
		ConsumerClient: consumerRedisClient,
		PubSubClient:   pubsubPool,
		PubsubAddr:     redisCfg.PubsubAddr,
	}

	if err := redisCache.PrimaryHealthCheck(ctx); err != nil {
		alog.Fatal("execute redis primary startup cmd", err)
	}
	if err := redisCache.PubsubHealthCheck(ctx); err != nil {
		alog.Fatal("execute redis pubsub startup cmd", err)
	}

	slog.Info("connected to redis node(s) successfully", "primaryClientKind", fmt.Sprintf("%T", redisClientClient))
	return redisCache
}

type RedisClusterConfig struct {
	ActiveProfile       config.Profile
	Nodes               []string
	PoolSize            int
	CredentialsProvider func() (string, string)
}

func NewUniversalClient(cfg RedisClusterConfig) redis.UniversalClient {
	var tlsConfig *tls.Config
	if cfg.ActiveProfile != config.Local {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:               cfg.Nodes,
		PoolSize:            cfg.PoolSize,
		DialTimeout:         5 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
		MaxRedirects:        10,
		CredentialsProvider: cfg.CredentialsProvider,
		TLSConfig:           tlsConfig,
	})
}
