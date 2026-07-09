package main

import (
	"context"
	"hexchess-lib/config"
	"hexchess-lib/dotenv"
	"hexchess-lib/logutil"
	"hexchess-svc/cloud"
	"hexchess-svc/controller"
	"hexchess-svc/db"
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
	rdbPrimaryUsername := os.Getenv("REDIS_SOR_USERNAME")
	rdbPrimaryPassword := os.Getenv("REDIS_SOR_PASSWORD")
	rdbPubSubNode := os.Getenv("REDIS_PUBSUB_NODE")
	rdbPubSubUsername := os.Getenv("REDIS_PUBSUB_USERNAME")
	rdbPubsubPassword := os.Getenv("REDIS_PUBSUB_PASSWORD")
	profile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	awsRegion := os.Getenv("AWS_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	awsUsername := os.Getenv("AWS_USERNAME")
	awsPassword := os.Getenv("AWS_PASSWORD")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	pdb := db.NewPostgresDB(ctx, db.PgPoolConfig{
		Dsn:           dbURL,
		ActiveProfile: profile,
		Region:        awsRegion,
	})
	defer pdb.Close()

	rdb := db.NewRedis(ctx, db.RedisConfig{
		PrimaryAddr:      rdbSorNodes,
		PrimaryUsername:  rdbPrimaryUsername,
		PrimaryPassword:  rdbPrimaryPassword,
		PubsubAddr:       rdbPubSubNode,
		PubsubUsername:   rdbPubSubUsername,
		PubsubPassword:   rdbPubsubPassword,
		ActiveProfile:    profile,
		ConsumerPoolSize: consumers.TotalPartitionCount,
	})
	defer rdb.Close()

	aws := cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
		ActiveProfile: profile,
		AWSRegion:     awsRegion,
		AWSEndpoint:   awsEndpoint,
		AWSUsername:   awsUsername,
		AWSPassword:   awsPassword,
	})
	remoteAPIs := cloud.NewRemoteAPIs(nil)
	broadcaster := pubsub.NewAsyncBroadcaster(rdb)

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
