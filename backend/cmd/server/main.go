package main

import (
	"context"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/internal/logutil"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	svc "hexchess-svc/service"
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
	rdbGameStoreNode := os.Getenv("REDIS_GAMESTORE_NODE")
	rdbCacheNodes := os.Getenv("REDIS_CACHE_NODES")
	rdbPubSubNode := os.Getenv("REDIS_PUBSUB_NODE")
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
		CacheAddr:     rdbCacheNodes,
		GameStoreAddr: rdbGameStoreNode,
		PubsubAddr:    rdbPubSubNode,
	}
	slog.Info("connecting to redis db", "addrs", addrs)
	rdb := db.MakeRedis(addrs, nil)

	aws, err := egress.MakeAwsClients(ctx, egress.AWSConfig{
		AWSDefaultRegion: awsDefaultRegion,
		AWSEndpoint:      awsEndpoint,
		IsLocalstack:     isLocalstack,
	})
	if err != nil {
		logutil.FatalErr("load aws config", err)
	}

	services := svc.MakeHexchessServices(svc.Setup{
		DB:          pdb,
		Redis:       rdb,
		AWS:         aws,
		Remote:      egress.MakeRemoteAPIs(),
		Broadcaster: pubsub.MakeBroadcaster(rdb),
	})
	defer services.Close()

	broadcasters := pubsub.MakeLocalBroadcasters()
	broadcasters.Listen(rdb)
	defer broadcasters.Shutdown()

	queue.StartRedisQueueConsumers(ctx, services, rdb)
	queue.StartDBQueueConsumers(ctx, services, pdb)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			slog.Error("failed while serving pprof", "error", err)
		}
	}()

	withHealthcheck := web.WithHealthCheckOpts(web.HealthCheckConfig{
		PostgresDSN:       dbURL,
		RedisGameStoreDSN: rdbGameStoreNode,
		RedisCacheDSN:     rdbCacheNodes,
		RedisPubSubDSN:    rdbPubSubNode,
	})
	serverSetup := web.Setup{
		Services:       services,
		AllowedOrigins: allowedOrigins,
		Broadcasters:   broadcasters,
	}
	if err := http.ListenAndServe(":"+serverPort, web.MakeServeMux(serverSetup, withHealthcheck)); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
