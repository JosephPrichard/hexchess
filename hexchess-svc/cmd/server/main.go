package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"hexchess-svc/app/dal"
	"hexchess-svc/db"
	"hexchess-svc/static"
	"log"
	"log/slog"
	"os"
	"strings"
)

func main() {
	initEnv()

	appPort := os.Getenv("APP_PORT")
	elasticUser := os.Getenv("ELASTICSEARCH_USERNAME")
	_ = os.Getenv("ELASTICSEARCH_PASSWORD")
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

	slog.Info("loaded environment variables",
		"APP_PORT", appPort,
		"ELASTICSEARCH_USERNAME", elasticUser,
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
	if err := json.Unmarshal(static.CountryListFile, &countryList); err != nil {
		log.Fatalf("failed to unmarshal country list: %v", err)
	}

	slog.Info("connecting to postgres db", "user", dbUser, "name", dbName, "port", dbPort)
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("user=%s dbname=%s password=%s port=%s", dbUser, dbName, dbPass, dbPort))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	pgDB := dal.MakeDbClient(q, pool)

	redisAddr := redisHost + ":" + redisPort
	psAddr := redisPubSubHost + ":" + redisPubSubPort

	slog.Info("connecting to redis db", "host", redisHost, "port", redisPort)
	rdb := dal.MakeRdbPool(redisAddr)

	slog.Info("connecting to redis pubsub channels", "host", redisPubSubHost, "port", redisPubSubPort)
	_ = dal.DialAndListenGameMessages(psAddr)

	_ = dal.Stores{Rdb: rdb, PgDB: pgDB}

	slog.Info("starting server", "port", appPort, "allowedOrigins", allowedOrigins)
}

func initEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Printf("error loading .env file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index < 0 {
			log.Fatalf("invalid line in .env file: %s", line)
		}
		key, value := line[:index], line[index+1:]
		if err := os.Setenv(key, value); err != nil {
			log.Printf("error setting env var: %v", err)
		}
	}
}
