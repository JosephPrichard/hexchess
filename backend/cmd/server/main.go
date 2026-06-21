package main

import (
	"context"
	"hexchess-svc/cloud"
	"hexchess-svc/controller"
	"hexchess-svc/db"
	"hexchess-svc/lib/dotenv"
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

const ServiceName = "hexchess-backend"

func main() {
	ctx := context.Background()

	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	dotenv.Load()

	serverPort := os.Getenv("SERVER_PORT")
	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	rdbPubSubNode := os.Getenv("REDIS_PUBSUB_NODE")
	profile := os.Getenv("PROFILE")
	awsRegion := os.Getenv("AWS_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	pool, closer := db.NewPgPool(ctx, db.PgConnectCfg{
		Dsn:     dbURL,
		Profile: profile,
		Region:  awsRegion,
	})
	defer closer()
	pdb := db.NewDB(pool)

	rdb, closer := db.NewRedis(ctx, db.RedisCfg{
		Addrs:   db.RedisAddrs{SorAddr: rdbSorNodes, PubsubAddr: rdbPubSubNode},
		Profile: profile,
	})
	defer closer()

	aws := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		Profile:     profile,
		AWSRegion:   awsRegion,
		AWSEndpoint: awsEndpoint,
	})
	remoteAPIs := cloud.NewRemoteAPIs(nil)
	broadcaster := pubsub.NewBroadcaster(rdb)

	services := svc.NewHexchessServices(svc.SetupService{
		DB:          pdb,
		Redis:       rdb,
		AWS:         aws,
		Remote:      remoteAPIs,
		Broadcaster: broadcaster,
	})

	broadcasters := pubsub.NewLocalBroadcasters()
	defer broadcasters.Shutdown()
	broadcasters.Listen(rdb)

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
		Broadcaster:    broadcaster,
		Broadcasters:   broadcasters,
		AllowedOrigins: allowedOrigins,
	}
	mux := controller.NewServeMux(serverSetup, withHealthcheck)

	if err := http.ListenAndServe(":"+serverPort, mux); err != nil {
		logutil.Fatal("failed while serving", err)
	}
}
