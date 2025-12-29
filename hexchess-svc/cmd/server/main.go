package main

import (
	"context"
	"encoding/json"
	"hexchess-svc/assets"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/logutil"
	svc "hexchess-svc/services"
	"hexchess-svc/web"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logutil.LogFatalErr("open log file", err)
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
	redisPrimaryURL := os.Getenv("REDIS_PRIMARY_URL")
	redisPubSubURL := os.Getenv("REDIS_PUBSUB_URL")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	pprofPort := os.Getenv("PPROF_PORT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	//cookieDomain := os.Getenv("COOKIE_DOMAIN")

	var countryList []string
	if err := json.Unmarshal(assets.CountryListJson, &countryList); err != nil {
		logutil.LogFatalErr("unmarshal country list", err)
	}

	slog.Info("connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		logutil.LogFatalErr("create pool", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), "SELECT 1;")
	if err != nil {
		logutil.LogFatalErr("execute startup query", err)
	}

	pdb := db.MakePostgres(pool)

	slog.Info("connecting to redis db", "primaryURL", redisPrimaryURL, "pubsubURL", redisPubSubURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: redisPrimaryURL, PubsubAddr: redisPubSubURL}, db.DefaultRedisNames)

	databases := db.Databases{Rdb: rdb, Pdb: pdb}
	defer databases.Close()

	state := web.State{
		Databases:      databases,
		Broadcasters:   svc.MakeBroadcaster(),
		Generators:     &outbound.RandomGenerator{},
		OutboundAPIs:   outbound.MakeRemoteAPIs(),
		CountryList:    countryList,
		AllowedOrigins: allowedOrigins,
	}
	<-state.Broadcasters.ListenGameMessages(rdb)
	<-state.Broadcasters.ListenUsersMessages(rdb)
	<-state.Broadcasters.ListenUnicastEvents(rdb)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	if pprofPort != "" {
		go func() {
			log.Println(http.ListenAndServe(":"+pprofPort, nil))
		}()
	}
	if err := http.ListenAndServe(":"+serverPort, web.HandleRoot(state)); err != nil {
		logutil.LogFatalErr("failed while serving", err)
	}
}
