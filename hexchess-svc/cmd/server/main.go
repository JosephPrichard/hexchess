package main

import (
	"context"
	"encoding/json"
	"hexchess-svc/assets"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/out"
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
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	pprofPort := os.Getenv("PPROF_PORT")
	// googleAPIKey := os.Getenv("GOOGLE_APIKEY")
	//cookieDomain := os.Getenv("COOKIE_DOMAIN")

	var countryList []string
	if err := json.Unmarshal(assets.CountryListJson, &countryList); err != nil {
		logutil.FatalErr("unmarshal country list", err)
	}

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

	pdb := db.MakePostgres(pool)

	slog.Info("connecting to rdb db", "primaryURL", rdbPrimaryURL, "pubsubURL", rdbPubSubURL)
	rdb := db.MakeRdb(db.RedisAddrs{CacheAddr: rdbPrimaryURL, PubsubAddr: rdbPubSubURL}, db.DefaultRedisNames)

	state := svc.State{Postgres: pdb, Redis: rdb, EntropySource: &out.NDEntropySource{}}
	defer state.Close()

	setup := web.Setup{
		State:          state,
		Broadcasters:   svc.MakeBroadcaster(),
		RemoteAPIs:     out.MakeRemoteAPIs(),
		CountryList:    countryList,
		AllowedOrigins: allowedOrigins,
	}
	<-setup.Broadcasters.ListenGameMessages(rdb)
	<-setup.Broadcasters.ListenUsersMessages(rdb)
	<-setup.Broadcasters.ListenUnicastEvents(rdb)

	slog.Info("starting server", "port", serverPort, "allowedOrigins", allowedOrigins)

	if pprofPort != "" {
		go func() {
			log.Println(http.ListenAndServe(":"+pprofPort, nil))
		}()
	}
	if err := http.ListenAndServe(":"+serverPort, web.MakeRoot(setup)); err != nil {
		logutil.FatalErr("failed while serving", err)
	}
}
