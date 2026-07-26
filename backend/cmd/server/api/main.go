package main

import (
	"context"
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
	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
	primaryDB := db.NewPostgresDB(ctx, db.PrimaryQuerierFactory, db.PoolConfig{
		Dsn:           cfg.PrimaryDbURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
	})
	defer primaryDB.Close()

	primaryRedis := db.NewRedis(ctx, db.RedisConfig{
		PrimaryAddr:     cfg.RedisPrimaryNodes,
		PrimaryUsername: cfg.RedisPrimaryUsername,
		PrimaryPassword: cfg.RedisPrimaryPassword,

		PubsubAddr:     cfg.RedisPubSubNode,
		PubsubUsername: cfg.RedisPubSubUsername,
		PubsubPassword: cfg.RedisPubsubPassword,

		ActiveProfile: cfg.Profile,
	})
	defer primaryRedis.Close()

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
	broadcaster := pubsub.NewAsyncBroadcaster(primaryRedis)

	// step 3: create API backend services and start background listeners for WS API
	services := svc.NewHexchessServices(svc.SetupService{
		PrimaryDB:   primaryDB,
		Redis:       primaryRedis,
		AWS:         aws,
		Remote:      remoteAPIs,
		Broadcaster: broadcaster,
	})

	broadcasters := pubsub.NewLocalBroadcasters()
	defer broadcasters.Shutdown()
	broadcasters.Listen(primaryRedis)

	// step 4: start API server and PPROF "sidecar" background task
	slog.Info("starting server", "port", cfg.ServerPort, "allowedOrigins", cfg.AllowedOrigins)

	withHealthcheck := controller.WithHealthCheckOpts(controller.HealthCheckConfig{
		PostgresDSN:       cfg.PrimaryDbURL,
		RedisGameStoreDSN: cfg.RedisPrimaryNodes,
		RedisCacheDSNs:    cfg.RedisPrimaryNodes,
		RedisPubSubDSN:    cfg.RedisPubSubNode,
	})
	serverSetup := controller.ServerSetup{
		Services:       services,
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: cfg.AllowedOrigins,
	}
	mux := controller.NewServeMux(serverSetup, withHealthcheck)

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()
	if err := http.ListenAndServe(":"+cfg.ServerPort, mux); err != nil {
		logutil.Fatal("failed while serving", err)
	}
}
