package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/consumers"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const ServiceName = "hexchess-consumer"

func main() {
	startTime := time.Now()

	slog.Info("begin consumer app")

	// parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := slogutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// connect to backend infrastructure and prepare cleanup
	databaseClient := database.NewDatabase(ctx, database.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL, // excludes opt read pool argument since all operations in this service involve mixed read-write operations
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer databaseClient.Close()

	riverPool := database.NewDatabasePool(ctx, database.PoolConfig{
		Dsn:           cfg.DbURL,
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer riverPool.Close()

	riverProducerClient := database.NewRiverClientFromPool(riverPool)
	defer riverProducerClient.Stop(ctx)

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:      cfg.RedisPrimaryNodes,
		PubsubAddr:       cfg.RedisPubSubNode,
		ActiveProfile:    cfg.Profile,
		ConsumerPoolSize: consumers.TotalPartitionCount,
	})
	defer redisClient.Close()

	// start background consumers and PPROF server
	broadcaster := pubsub.NewAsyncBroadcaster(redisClient)

	consumers.StartRedisConsumers(consumers.RedisConsumerConfig{
		Database:    databaseClient,
		Redis:       redisClient,
		Broadcaster: broadcaster,
		RiverClient: riverProducerClient,
	})
	consumers.StartRiverConsumers(consumers.RiverConsumerConfig{
		PGXPool:     riverPool,
		Database:    databaseClient,
		Redis:       redisClient,
		Broadcaster: broadcaster,
	})

	slog.Info("finished initializing consumers", "timeTaken", time.Since(startTime).String())

	if err := http.ListenAndServe(":6060", nil); err != nil {
		slog.Error("failed while serving pprof", "error", err)
	}
}
