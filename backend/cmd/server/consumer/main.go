package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/db"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/consumers"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const ServiceName = "hexchess-consumer"

func main() {
	startTime := time.Now()

	slog.Info("begin consumer app")

	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
	database := db.NewDatabase(ctx, db.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL, // excludes optional read pool argument since all operations in this service involve mixed read-write operations
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer database.Close()

	riverQuePool := db.NewDatabasePool(ctx, db.PoolConfig{
		Dsn:           cfg.DbURL,
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer riverQuePool.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:      cfg.RedisPrimaryNodes,
		PubsubAddr:       cfg.RedisPubSubNode,
		ActiveProfile:    cfg.Profile,
		ConsumerPoolSize: consumers.TotalPartitionCount,
	})
	defer redisClient.Close()

	// step 3: start background consumers and PPROF server
	broadcaster := pubsub.NewAsyncBroadcaster(redisClient)

	services := svc.NewHexchessServices(svc.SetupService{
		Database:    database,
		Redis:       redisClient,
		Broadcaster: broadcaster,
	})

	consumers.StartRedisConsumers(consumers.RedisConsumerSetup{
		Database: database,
		Redis:    redisClient,
		Services: services,
	})
	consumers.StartRiverConsumers(consumers.RiverConsumerSetup{
		PgxPool:  riverQuePool,
		Services: services,
	})

	slog.Info("finished initializing consumers", "timeTaken", time.Since(startTime).String())

	if err := http.ListenAndServe(":6060", nil); err != nil {
		slog.Error("failed while serving pprof", "error", err)
	}
}
