package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/data"
	"hexchess-svc/db"
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
		util.LogFatalErr("failed to open log file", err)
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
	//cookieDomain := os.Getenv("COOKIE_DOMAIN")

	slog.Info("loaded environment variables", "envs", envMap)

	var countryList []string
	if err := json.Unmarshal(static.CountryListJson, &countryList); err != nil {
		util.LogFatalErr("failed to unmarshal country list", err)
	}

	slog.Info("connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		util.LogFatalErr("failed to create pool", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), "SELECT 1;")
	if err != nil {
		util.LogFatalErr("failed to execute startup query", err)
	}

	q := db.New(pool)
	pgDB := data.MakeDbClient(q, pool)

	slog.Info("connecting to redis db", "primaryURL", redisPrimaryURL, "pubsubURL", redisPubSubURL)
	rdb := data.MakeRdb(redisPrimaryURL, redisPubSubURL)
	defer rdb.Close()

	state := web.MakeServerState(data.Stores{Rdb: rdb, PgDB: pgDB}, countryList)

	data.ListenGameMessages(state.GamesCaster, rdb.PubsubAddr)
	data.ListenUsersMessages(state.UsersCaster, rdb.PubsubAddr)
	data.ListenUnicastEvents(state.CountsCaster, rdb.PubsubAddr)

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
