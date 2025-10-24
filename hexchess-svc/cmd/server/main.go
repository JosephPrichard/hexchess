package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/cmd"
	"hexchess-svc/data"
	"hexchess-svc/db"
	"hexchess-svc/lib"
	"hexchess-svc/static"
	"log"
	"log/slog"
	"os"
)

func main() {
	ctx := context.WithValue(context.Background(), "trace", "server-init")

	f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	lib.InitLogger(f)
	cmd.InitEnv()

	appPort := os.Getenv("APP_PORT")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPubSubHost := os.Getenv("REDIS_PUBSUB_HOST")
	redisPubSubPort := os.Getenv("REDIS_PUBSUB_PORT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	cookieDomain := os.Getenv("COOKIE_DOMAIN")

	slog.InfoContext(ctx, "loaded environment variables",
		"APP_PORT", appPort,
		"DB_NAME", dbName,
		"DB_PORT", dbPort,
		"DB_USER", dbUser,
		"REDIS_HOST", redisHost,
		"REDIS_PORT", redisPort,
		"REDIS_PUBSUB_HOST", redisPubSubHost,
		"REDIS_PUBSUB_PORT", redisPubSubPort,
		"ALLOWED_ORIGINS", allowedOrigins,
		"COOKIE_DOMAIN", cookieDomain,
	)

	var countryList []string
	if err := json.Unmarshal(static.CountryListJson, &countryList); err != nil {
		log.Fatalf("failed to unmarshal country list: %v", err)
	}

	slog.InfoContext(ctx, "connecting to postgres db", "user", dbUser, "name", dbName, "port", dbPort)
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%s", dbUser, dbName, dbPass, dbPort))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	pgDB := data.MakeDbClient(q, pool)

	redisAddr := redisHost + ":" + redisPort
	//psAddr := redisPubSubHost + ":" + redisPubSubPort

	slog.InfoContext(ctx, "connecting to redis db", "host", redisHost, "port", redisPort)
	rdb := data.MakeRdb(redisAddr)
	defer rdb.Close()

	slog.InfoContext(ctx, "connecting to redis pubsub channels", "host", redisPubSubHost, "port", redisPubSubPort)
	//_ = data.ListenGameMessages(psAddr)

	_ = data.Stores{Rdb: rdb, PgDB: pgDB}

	slog.InfoContext(ctx, "starting server", "port", appPort, "allowedOrigins", allowedOrigins)
}
