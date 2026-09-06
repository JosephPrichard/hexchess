package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/database"
	"hexchess-svc/network"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const ServiceName = "hexchess-api"

func main() {
	startTime := time.Now()

	slog.Info("begin api app")

	// parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := slogutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// connect to backend infrastructure and prepare cleanup
	databaseClient := database.NewDatabase(ctx, database.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL,
		ReadDsn:       cfg.DbReadURL, // provides read pool for increased performance
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer databaseClient.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		PubsubAddr:    cfg.RedisPubSubNode,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	riverClient := database.NewRiverClient(ctx, database.PoolConfig{
		Dsn:           cfg.DbURL,
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer riverClient.Stop(ctx)

	aws := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		ActiveProfile: cfg.Profile,
		Names:         cloud.AWSNames{S3ProfileBucket: cfg.ProfileBucket},
		AWSRegion:     cfg.AwsRegion,
		AWSEndpoint:   cfg.AwsEndpoint,
		AWSUsername:   cfg.AwsUsername,
		AWSPassword:   cfg.AwsPassword,
	})
	remoteAPIs := cloud.NewRemoteAPIs(nil)
	broadcaster := pubsub.NewAsyncBroadcaster(redisClient)

	broadcasters := pubsub.NewLocalBroadcasters()
	broadcasters.Listen(redisClient)

	// start API server and PPROF "sidecar" background task
	mux := network.NewServeMux(network.HttpServerConfig{
		Database:       databaseClient,
		RiverClient:    riverClient,
		Redis:          redisClient,
		AWS:            aws,
		SDKs:           remoteAPIs,
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: cfg.AllowedOrigins,
	}, network.NewHealthCheck(network.HealthConfig{
		PostgresCheck:     databaseClient.HealthcheckFunc(),
		PostgresReadCheck: databaseClient.ReadHealthcheckFunc(),
		RedisPrimaryCheck: redisClient.PrimaryHealthCheck,
		RedisPubSubCheck:  redisClient.PubsubHealthCheck,
	}))

	slog.Info("finished initializing app, starting server",
		"port", cfg.ServerPort, "allowedOrigins", cfg.AllowedOrigins, "timeTaken", time.Since(startTime).String())

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()
	if err := http.ListenAndServe(":"+cfg.ServerPort, mux); err != nil {
		slogutil.Fatal("failed while serving", err)
	}
}
