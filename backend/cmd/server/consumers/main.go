package main

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/queue/consumers"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/dotenv"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
)

const ServiceName = "hexchess-consumer"

func main() {
	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	dotenv.Load()

	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	rdbPrimaryUsername := os.Getenv("REDIS_SOR_USERNAME")
	rdbPrimaryPassword := os.Getenv("REDIS_SOR_PASSWORD")
	rdbPubSubNode := os.Getenv("REDIS_PUBSUB_NODE")
	rdbPubSubUsername := os.Getenv("REDIS_PUBSUB_USERNAME")
	rdbPubsubPassword := os.Getenv("REDIS_PUBSUB_PASSWORD")
	profile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	awsRegion := os.Getenv("AWS_REGION")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
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

	// step 3: start background consumers and PPROF server
	services := svc.NewHexchessServices(svc.SetupService{
		DB:    pdb,
		Redis: rdb,
	})
	consumers.StartConsumers(consumers.SetupConsumers{
		Ctx:      ctx,
		Services: services,
		Postgres: pdb,
		Redis:    rdb,
	})

	if err := http.ListenAndServe(":6060", nil); err != nil {
		slog.Error("failed while serving pprof", "error", err)
	}
}
