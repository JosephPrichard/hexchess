package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"hexchess-svc/config"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type slogAdapter struct{}

func (a slogAdapter) Printf(ctx context.Context, format string, v ...any) {
	slog.LogAttrs(ctx, slog.LevelInfo, fmt.Sprintf(format, v...), slog.String("component", "go-redis"))
}

func init() {
	redis.SetLogger(slogAdapter{})
}

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

// TotalConsumerPartitionCount is the total number of partitions created by ALL Redis stream consumers
// since consumers block an entire connection while reading
// we need a separate pool with the pool size set to the expected number of consumers = (streams * partitions_per_steam)
// it is computed manually to avoid circular dependencies - consumer should check this value and fail-fast if it creates more consumers than expected
var (
	TotalConsumerPartitionCount  = GameJobConsumerCount * len(PartitionsPerGameJobConsumer)
	GameJobConsumerCount         = 2
	PartitionsPerGameJobConsumer = gameid.GameIDPartitions()
)

type Redis struct {
	PrimaryClient  redis.UniversalClient
	ConsumerClient redis.UniversalClient
	PubSubClient   *redis.Client

	TotalPartitionCount int `json:"totalPartitionCount"`
}

func (redis *Redis) PrimaryHealthCheck(ctx context.Context) error {
	err := redis.PrimaryClient.Ping(ctx).Err()
	if err != nil {
		slog.ErrorContext(ctx, "healthcheck error", "error", err, "kind", "primaryRedis")
	}
	return err
}

func (redis *Redis) PubsubHealthCheck(ctx context.Context) error {
	err := redis.PubSubClient.Ping(ctx).Err()
	if err != nil {
		slog.ErrorContext(ctx, "healthcheck error", "error", err, "kind", "pubSubRedis")
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
	slog.Info("creating redis clients", "config", redisCfg, "totalConsumerPartitionCount", TotalConsumerPartitionCount)

	pubsubClient := NewSimpleClient(redisCfg.PubsubAddr)

	redisPrimaryClient := NewUniversalClient(RedisClusterConfig{
		ActiveProfile: redisCfg.ActiveProfile,
		Nodes:         redisCfg.PrimaryAddr,
		PoolSize:      0, // default pool size
		Username:      redisCfg.PrimaryUsername,
		Password:      redisCfg.PrimaryPassword,
	})
	consumerRedisClient := NewUniversalClient(RedisClusterConfig{
		ActiveProfile: redisCfg.ActiveProfile,
		Nodes:         redisCfg.PrimaryAddr,
		PoolSize:      TotalConsumerPartitionCount,
		Username:      redisCfg.PrimaryUsername,
		Password:      redisCfg.PrimaryPassword,
	})

	redisCache := Redis{
		PrimaryClient:       redisPrimaryClient,
		ConsumerClient:      consumerRedisClient,
		PubSubClient:        pubsubClient,
		TotalPartitionCount: TotalConsumerPartitionCount,
	}

	if err := redisCache.PrimaryHealthCheck(ctx); err != nil {
		slogutil.Fatal("execute redis primary startup cmd", err)
	}
	if err := redisCache.PubsubHealthCheck(ctx); err != nil {
		slogutil.Fatal("execute redis pubsub startup cmd", err)
	}

	slog.Info("connected to redis node(s) successfully", "primaryClientKind", fmt.Sprintf("%T", redisPrimaryClient))
	return redisCache
}

func NewSimpleClient(connURL string) *redis.Client {
	// no TLS config + credentials provider: ElastiCache connectivity is often done without auth: we're relying on firewalls + private subnet to keep our data secure
	return redis.NewClient(&redis.Options{
		Addr:         connURL,
		PoolSize:     0, // default pool size
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
}

type RedisClusterConfig struct {
	ActiveProfile config.Profile
	Nodes         []string
	PoolSize      int
	Username      string
	Password      string
}

func NewUniversalClient(cfg RedisClusterConfig) redis.UniversalClient {
	var credsProvider func() (string, string)
	if cfg.ActiveProfile != config.Local {
		credsProvider = func() (string, string) {
			return cfg.Username, cfg.Password
		}
	}

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
		CredentialsProvider: credsProvider,
		TLSConfig:           tlsConfig,
	})
}
