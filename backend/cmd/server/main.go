package main

import (
	"context"
	"hexchess-svc/cmd"
	"hexchess-svc/controller"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/consumers"
	svc "hexchess-svc/service"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
)

func main() {
	ctx := context.Background()

	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	cmd.InitEnv()

	serverPort := os.Getenv("SERVER_PORT")
	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	rdbPubSubNode := os.Getenv("REDIS_PUBSUB_NODE")
	isLocalAWS := os.Getenv("IS_LOCAL_AWS") == "true"
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	oltpEndpoint := os.Getenv("OLTP_ENDPOINT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	shutdown := logutil.InitLoggers(logutil.LogConfig{OtlpEndpoint: oltpEndpoint})
	defer shutdown()

	pdb := db.MakeDB(db.MakePgPool(ctx, dbURL))
	defer pdb.Close()

	rdb := db.MakeRedis(db.RedisAddrs{
		SorAddr:    rdbSorNodes,
		PubsubAddr: rdbPubSubNode,
	}, nil)
	defer rdb.Close()

	aws, err := egress.MakeAWSClients(ctx, egress.AWSConfig{
		AWSDefaultRegion:  awsDefaultRegion,
		AWSEndpoint:       awsEndpoint,
		IsTestCredentials: isLocalAWS,
	}, nil)
	if err != nil {
		logutil.FatalErr("make aws clients", err)
	}

	remoteAPIs := egress.MakeRemoteAPIs(nil)

	services := svc.MakeHexchessServices(svc.SetupService{
		DB:          pdb,
		Redis:       rdb,
		AWS:         aws,
		Remote:      remoteAPIs,
		Broadcaster: pubsub.MakeBroadcaster(rdb),
	})

	broadcasters := pubsub.MakeLocalBroadcasters()
	broadcasters.Listen(rdb)
	defer broadcasters.Shutdown()

	consumers.StartConsumers(consumers.SetupConsumers{
		Ctx:      ctx,
		Services: services,
		Postgres: pdb,
		Redis:    rdb,
	})

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()

	withHealthcheck := controller.WithHealthCheckOpts(controller.HealthCheckConfig{
		PostgresDSN:       dbURL,
		RedisGameStoreDSN: rdbSorNodes,
		RedisCacheDSNs:    rdbSorNodes,
		RedisPubSubDSN:    rdbPubSubNode,
	})
	serverSetup := controller.ServerSetup{
		Services:       services,
		Broadcaster:    pubsub.MakeBroadcaster(rdb),
		Broadcasters:   broadcasters,
		AllowedOrigins: allowedOrigins,
	}
	mux := controller.MakeServeMux(serverSetup, withHealthcheck)

	if err := http.ListenAndServe(":"+serverPort, mux); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
