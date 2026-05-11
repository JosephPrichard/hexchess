package main

import (
	"context"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/queue"
	svc "hexchess-svc/service"
	"hexchess-svc/util/logutil"
	"hexchess-svc/web"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	cmd.InitEnv()

	serverPort := os.Getenv("SERVER_PORT")
	dbURL := os.Getenv("DB_URL")
	rdbGameStoreURL := os.Getenv("REDIS_GAMESTORE_URL")
	rdbCacheURL := os.Getenv("REDIS_CACHE_URL")
	rdbPubSubURL := os.Getenv("REDIS_PUBSUB_URL")
	isLocalstack := os.Getenv("IS_LOCALSTACK") == "true"
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	oltpEndpoint := os.Getenv("OLTP_ENDPOINT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	shutdown := logutil.InitLoggers(logutil.LogConfig{OtlpEndpoint: oltpEndpoint})
	defer shutdown(ctx)

	slog.Info("connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, "SELECT 1;")
	if err != nil {
		logutil.FatalErr("execute startup query", err)
	}

	pdb := db.MakeDB(pool)

	addrs := db.RedisAddrs{
		CacheAddr:     rdbCacheURL,
		GameStoreAddr: rdbGameStoreURL,
		PubsubAddr:    rdbPubSubURL,
	}
	slog.Info("connecting to redis db", "addrs", addrs)
	rdb := db.MakeRdb(addrs, nil)

	aws, err := egress.MakeAwsClients(ctx, egress.AWSConfig{
		AWSDefaultRegion: awsDefaultRegion,
		AWSEndpoint:      awsEndpoint,
		IsLocalstack:     isLocalstack,
	})
	if err != nil {
		logutil.FatalErr("load aws config", err)
	}

	services := svc.MakeHexchessServices(svc.Setup{
		DB:     pdb,
		Redis:  rdb,
		AWS:    aws,
		Remote: egress.MakeRemoteAPIs(),
	})
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	broadcasters.Listen(rdb)
	defer broadcasters.Shutdown()

	queue.StartRedisQueueConsumers(ctx, services, rdb)
	queue.StartDBQueueConsumers(ctx, services, pdb)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "err", err)
		}
	}()

	withHealthcheck := web.WithHealthCheckOpts(web.HealthCheckConfig{
		PostgresDSN:     dbURL,
		RedisPrimaryDSN: rdbCacheURL,
		RedisPubSubDSN:  rdbPubSubURL,
	})
	serverSetup := web.Setup{
		Services:       services,
		AllowedOrigins: allowedOrigins,
		Broadcasers:    broadcasters,
	}
	if err := http.ListenAndServe(":"+serverPort, web.MakeServeMux(serverSetup, withHealthcheck)); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
