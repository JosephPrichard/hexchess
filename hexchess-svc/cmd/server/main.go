package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"hexchess-svc/service"
	"hexchess-svc/util/logutil"
	"hexchess-svc/web"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logutil.FatalErr("open log file", err)
	}
	defer f.Close()

	logutil.InitLoggers(f)
	cmd.InitEnv()

	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		envMap[pair[0]] = pair[1]
	}

	serverPort := os.Getenv("SERVER_PORT")
	dbURL := os.Getenv("DB_URL")
	rdbGameStoreURL := os.Getenv("REDIS_GAMESTORE_URL")
	rdbCacheURL := os.Getenv("REDIS_CACHE_URL")
	rdbPubSubURL := os.Getenv("REDIS_PUBSUB_URL")
	isLocalstack := os.Getenv("IS_LOCALSTACK") == "true"
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	pprofPort := os.Getenv("PPROF_PORT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	// cookieDomain := os.Getenv("COOKIE_DOMAIN")

	slog.Info("connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), "SELECT 1;")
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

	aws, err := egress.MakeAwsClients(context.Background(), egress.AWSConfig{
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

	<-broadcasters.ListenGameMessages(rdb)
	<-broadcasters.ListenUsersMessages(rdb)
	<-broadcasters.ListenUnicastEvents(rdb)

	svc.StartStreamReaders(context.Background(), services)
	svc.StartOutboxQueueConsumers(context.Background(), services)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	if pprofPort != "" {
		go func() {
			if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
				slog.Error("failed while serving pprof", "err", err)
			}
		}()
	}

	mux := web.MakeServeMux(web.Setup{
		Services:       services,
		AllowedOrigins: allowedOrigins,
	})
	web.WithHealthCheck(mux, web.HealthCheckConfig{
		PostgresDSN:     dbURL,
		RedisPrimaryDSN: rdbCacheURL,
		RedisPubSubDSN:  rdbPubSubURL,
	})
	if err := http.ListenAndServe(":"+serverPort, mux); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
