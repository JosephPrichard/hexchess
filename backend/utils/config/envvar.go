package config

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	ServerPort           string
	PrimaryDbURL         string
	MetricsDbURL         string
	RedisPrimaryNodes    []string
	RedisPrimaryUsername string
	RedisPrimaryPassword string
	RedisPubSubNode      string
	RedisPubSubUsername  string
	RedisPubsubPassword  string
	Profile              Profile
	AwsRegion            string
	AwsEndpoint          string
	AwsUsername          string
	AwsPassword          string
	AllowedOrigins       string
	OltpEndpoint         string
}

func loadDotenv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			slog.Error("invalid line in .env file", nil, "line", line)
			os.Exit(1)
		}
		// these environment variables last until the end of this process
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("error setting env var", "error", err)
		}
	}
}

func loadConfig() Config {
	return Config{
		ServerPort:           os.Getenv("SERVER_PORT"),
		PrimaryDbURL:         os.Getenv("PRIMARY_DB_URL"),
		MetricsDbURL:         os.Getenv("METRICS_DB_URL"),
		RedisPrimaryNodes:    strings.Split(os.Getenv("REDIS_SOR_NODES"), ","),
		RedisPrimaryUsername: os.Getenv("REDIS_SOR_USERNAME"),
		RedisPrimaryPassword: os.Getenv("REDIS_SOR_PASSWORD"),
		RedisPubSubNode:      os.Getenv("REDIS_PUBSUB_NODE"),
		RedisPubSubUsername:  os.Getenv("REDIS_PUBSUB_USERNAME"),
		RedisPubsubPassword:  os.Getenv("REDIS_PUBSUB_PASSWORD"),
		Profile:              ParseProfile(os.Getenv("ACTIVE_PROFILE")),
		AwsRegion:            os.Getenv("AWS_REGION"),
		AwsEndpoint:          os.Getenv("AWS_ENDPOINT"),
		AwsUsername:          os.Getenv("AWS_USERNAME"),
		AwsPassword:          os.Getenv("AWS_PASSWORD"),
		AllowedOrigins:       os.Getenv("ALLOWED_ORIGINS"),
		OltpEndpoint:         os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
}

func Load() Config {
	loadDotenv()
	return loadConfig()
}
