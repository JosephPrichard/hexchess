package main

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/metricsdb"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/queue/consumers"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
)

const ServiceName = "hexchess-consumer"

func main() {
	// step 1: parse CLI inputs for static input data
	ctx := context.Background()

	cfg := config.Load()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure and prepare cleanup
	primaryDB := db.NewPostgresDB(ctx, db.PoolConfig[primarydb.Querier]{
		Dsn:           cfg.PrimaryDbURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
		Factory:       db.PrimaryQuerierFactory,
	})
	metricsDB := db.NewPostgresDB(ctx, db.PoolConfig[metricsdb.Querier]{
		Dsn:           cfg.MetricsDbURL,
		ActiveProfile: cfg.Profile,
		Region:        cfg.AwsRegion,
		Factory:       db.MetricsQuerierFactory,
	})
	defer primaryDB.Close()

	primaryRedis := db.NewRedis(ctx, db.RedisConfig{
		PrimaryAddr:      cfg.RedisPrimaryNodes,
		PrimaryUsername:  cfg.RedisPrimaryUsername,
		PrimaryPassword:  cfg.RedisPrimaryPassword,
		PubsubAddr:       cfg.RedisPubSubNode,
		PubsubUsername:   cfg.RedisPubSubUsername,
		PubsubPassword:   cfg.RedisPubsubPassword,
		ActiveProfile:    cfg.Profile,
		ConsumerPoolSize: consumers.TotalPartitionCount,
	})
	defer primaryRedis.Close()

	// step 3: start background consumers and PPROF server
	services := svc.NewHexchessServices(svc.SetupService{
		PrimaryDB: primaryDB,
		Redis:     primaryRedis,
	})
	consumers.StartConsumers(consumers.SetupConsumers{
		Ctx:       ctx,
		Services:  services,
		PrimaryDB: primaryDB,
		MetricsDB: metricsDB,
		Redis:     primaryRedis,
	})

	if err := http.ListenAndServe(":6060", nil); err != nil {
		slog.Error("failed while serving pprof", "error", err)
	}
}
