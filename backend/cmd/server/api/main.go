package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/config"
	"hexchess-svc/database"
	"hexchess-svc/featureflag"
	"hexchess-svc/network"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/queue/consumers"
	"hexchess-svc/trigger"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const ServiceName = "hexchess-api"

func main() {
	startTime := time.Now()

	slog.Info("start server")

	// parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := slogutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	featureflag.InitS3Client(ctx, cfg.Profile, cfg.AwsRegion)

	// connect to backend infrastructure and prepare cleanup
	databasePools := database.NewDatabasePools(ctx, database.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL,
		ReadDsn:       cfg.DbReadURL, // provides read pool for increased performance
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})

	databaseClient := database.NewDatabase(databasePools)
	defer databaseClient.Close()

	riverClient := database.NewRiverClient(databasePools.Write)
	defer riverClient.Stop(ctx)

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		PubsubAddr:    cfg.RedisPubSubNode,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	awsClient := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		ActiveProfile: cfg.Profile,
		Names:         cloud.AWSNames{S3ProfileBucket: cfg.ProfileBucket},
		AWSRegion:     cfg.AwsRegion,
		AWSEndpoint:   cfg.AwsEndpoint,
		AWSUsername:   cfg.AwsUsername,
		AWSPassword:   cfg.AwsPassword,
	})
	remoteAPIs := cloud.NewRemoteAPIs(nil)

	esClient := cloud.NewElasticSearchClient(cloud.EsConfig{
		ActiveProfile: cfg.Profile,
		Address:       cfg.EsEndpoint,
		CreateIndices: true,
	})
	if esClient != nil {
		defer esClient.Close(ctx)
	}

	broadcaster := publishers.NewAsyncBroadcaster(redisClient)

	broadcasters := subscribers.NewLocalBroadcasters()
	broadcasters.Listen(redisClient)

	// start API server and PPROF "sidecar" background task
	mux := network.NewServeMux(network.HttpServerConfig{
		Database:       databaseClient,
		RiverClient:    riverClient,
		Redis:          redisClient,
		AWS:            awsClient,
		EsClient:       esClient,
		SDKs:           remoteAPIs,
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	// start background job consumers
	consumers.StartRedisConsumers(consumers.RedisConsumerConfig{
		Database:    databaseClient,
		Redis:       redisClient,
		Broadcaster: broadcaster,
		RiverClient: riverClient,
	})
	consumers.StartRiverConsumers(consumers.RiverConsumerConfig{
		PGXPool:     databasePools.Write,
		Database:    databaseClient,
		Redis:       redisClient,
		EsClient:    esClient,
		Broadcaster: broadcaster,
	})

	// start background trigger handlers
	trigger.Execute(nil, trigger.TriggerConfig{RedisClient: redisClient})

	slog.Info("finished initializing app, starting server", "port", cfg.ServerPort, "allowedOrigins", cfg.AllowedOrigins, "timeTaken", time.Since(startTime).String())

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()
	if err := http.ListenAndServe(":"+cfg.ServerPort, mux); err != nil {
		slogutil.Fatal("failed while serving", err)
	}
}
