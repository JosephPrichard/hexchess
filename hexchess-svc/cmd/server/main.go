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
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
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
	rdbPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")
	rdbPubSubURL := os.Getenv("REDIS_PUBSUB_URL")
	awsSecretID := os.Getenv("AWS_SECRET_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_KEY")
	awsDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	pprofPort := os.Getenv("PPROF_PORT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	//cookieDomain := os.Getenv("COOKIE_DOMAIN")

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

	slog.Info("connecting to rdb db", "primaryURL", rdbPrimaryURL, "pubsubURL", rdbPubSubURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL, PubsubAddr: rdbPubSubURL}, db.DefaultRedisNames)

	aws, err := egress.MakeAwsClients(context.Background(), egress.AWSConfig{
		AWSDefaultRegion: awsDefaultRegion,
		AWSSecretKey:     awsSecretKey,
		AWSSecretID:      awsSecretID,
		AWSEndpoint:      awsEndpoint,
	})
	if err != nil {
		logutil.FatalErr("load aws config", err)
	}

	services := svc.Services{
		DB:            pdb,
		Queries:       pdb.Queries(),
		Redis:         rdb,
		AWS:           aws,
		Broadcasters:  svc.MakeBroadcasters(),
		Remote:        egress.MakeRemoteAPIs(),
		EntropySource: &svc.RealEntropySource{},
	}
	defer services.Close()

	<-services.Broadcasters.ListenGameMessages(rdb)
	<-services.Broadcasters.ListenUsersMessages(rdb)
	<-services.Broadcasters.ListenUnicastEvents(rdb)

	svc.StartStreamReaders(context.Background(), &services)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	if pprofPort != "" {
		go func() {
			if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
				slog.Error("failed while serving pprof", "err", err)
			}
		}()
	}

	setup := web.Setup{
		Services:       services,
		AllowedOrigins: allowedOrigins,
	}

	mux := web.MakeServeMux(setup)
	web.WithHealthCheck(mux, web.HealthCheckConfig{
		PostgresDSN:     dbURL,
		RedisPrimaryDSN: rdbPrimaryURL,
		RedisPubSubDSN:  rdbPubSubURL,
	})
	if err := http.ListenAndServe(":"+serverPort, mux); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
