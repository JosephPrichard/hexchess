package main

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/network"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const ServiceName = "hexchess-api"

func main() {
	startTime := time.Now()

	slog.Info("begin api app")

	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
	database := db.NewDatabase(ctx, db.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL,
		ReadDsn:       cfg.DbReadURL, // provides read pool for increased performance
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer database.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		PubsubAddr:    cfg.RedisPubSubNode,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

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

	// step 3: create API backend services and start background listeners for WS API
	services := svc.NewHexchessServices(svc.SetupService{
		Database:    database,
		Redis:       redisClient,
		AWS:         aws,
		SDKs:        remoteAPIs,
		Broadcaster: broadcaster,
	})

	broadcasters := pubsub.NewLocalBroadcasters()
	broadcasters.Listen(redisClient)

	// step 4: start API server and PPROF "sidecar" background task
	mux := network.NewServeMux(network.ServerSetup{
		Services:       services,
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: cfg.AllowedOrigins,
	}, network.NewHealthCheck(network.HealthConfig{
		PostgresCheck:     database.HealthcheckFunc(),
		PostgresReadCheck: database.ReadHealthcheckFunc(),
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
		logutil.Fatal("failed while serving", err)
	}
}
