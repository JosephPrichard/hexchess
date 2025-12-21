package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/services"
	"hexchess-svc/static"
	"hexchess-svc/util"
	"hexchess-svc/web"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
)

func main() {
	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		util.LogFatalErr("open log file", err)
	}
	defer f.Close()

	util.InitLoggers(f)
	util.InitEnv()

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
	if err := json.Unmarshal(static.CountryListJson, &countryList); err != nil {
		util.LogFatalErr("unmarshal country list", err)
	}

	slog.Info("connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		util.LogFatalErr("create pool", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), "SELECT 1;")
	if err != nil {
		util.LogFatalErr("execute startup query", err)
	}

	q := db.New(pool)
	postgres := db.MakePostgres(q, pool)

	slog.Info("connecting to redis db", "primaryURL", redisPrimaryURL, "pubsubURL", redisPubSubURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: redisPrimaryURL, PubsubAddr: redisPubSubURL}, db.DefaultRedisNames)
	defer rdb.Close()

	state := web.MakeServerState(web.ServerSetup{
		Databases: db.Databases{
			Rdb: rdb,
			Pdb: postgres,
		},
		CountryList: countryList,
		Generators:  &web.RandGenerator{},
		OutboundAPIs: outbound.APIs{
			GoogleAPI: &outbound.RemoteGoogleAPI{},
		},
	})

	<-svc.ListenGameMessages(state.GamesCaster, rdb)
	<-svc.ListenUsersMessages(state.UsersCaster, rdb)
	<-svc.ListenUnicastEvents(state.CountsCaster, rdb)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	if pprofPort != "" {
		go func() {
			log.Println(http.ListenAndServe(":"+pprofPort, nil))
		}()
	}
	if err := http.ListenAndServe(":"+serverPort, web.HandleRoot(state, allowedOrigins)); err != nil {
		util.LogFatalErr("failed while serving", err)
	}
}
