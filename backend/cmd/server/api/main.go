package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/controller"
	"hexchess-svc/db"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
)

const ServiceName = "hexchess-api"

func main() {
	slog.Info("begin api app")

	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
	database := db.NewDatabase(ctx, db.DatabaseConfig{
		Dsn:           cfg.DbURL,
		ReadOnlyDsn:   cfg.DbReadURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer database.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:     cfg.RedisPrimaryNodes,
		PrimaryUsername: cfg.RedisPrimaryUsername,
		PrimaryPassword: cfg.RedisPrimaryPassword,

		PubsubAddr:     cfg.RedisPubSubNode,
		PubsubUsername: cfg.RedisPubSubUsername,
		PubsubPassword: cfg.RedisPubsubPassword,

		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	aws := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		ActiveProfile: cfg.Profile,
		Names: cloud.AWSNames{
			S3ProfileBucket: cfg.ProfileBucket,
		},
		AWSRegion:   cfg.AwsRegion,
		AWSEndpoint: cfg.AwsEndpoint,
		AWSUsername: cfg.AwsUsername,
		AWSPassword: cfg.AwsPassword,
	})
	remoteAPIs := cloud.NewRemoteAPIs(nil)
	broadcaster := pubsub.NewAsyncBroadcaster(redisClient)

	// step 3: create API backend services and start background listeners for WS API
	services := svc.NewHexchessServices(svc.SetupService{
		Database:    database,
		Redis:       redisClient,
		AWS:         aws,
		Remote:      remoteAPIs,
		Broadcaster: broadcaster,
	})

	broadcasters := pubsub.NewLocalBroadcasters()
	defer broadcasters.Shutdown()
	broadcasters.Listen(redisClient)

	// step 4: start API server and PPROF "sidecar" background task
	slog.Info("starting server", "port", cfg.ServerPort, "allowedOrigins", cfg.AllowedOrigins)

	mux := controller.NewServeMux(controller.ServerSetup{
		Services:       services,
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: cfg.AllowedOrigins,
	}, controller.NewHealthCheck(controller.HealthConfig{
		PostgresCheck:     database.HealthcheckFunc(),
		PostgresReadCheck: database.ReadHealthcheckFunc(),
		RedisPrimaryCheck: redisClient.PrimaryHealthCheck,
		RedisPubSubCheck:  redisClient.PubsubHealthCheck,
	}))

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()
	if err := http.ListenAndServe(":"+cfg.ServerPort, mux); err != nil {
		logutil.Fatal("failed while serving", err)
	}
}
