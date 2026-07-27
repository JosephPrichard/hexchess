package config

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	ServerPort           string   `json:"serverPort"`
	DbURL                string   `json:"dbURL"`
	RedisPrimaryNodes    []string `json:"redisPrimaryNodes"`
	RedisPrimaryUsername string   `json:"redisPrimaryUsername"`
	RedisPrimaryPassword string   `json:"-"`
	RedisPubSubNode      string   `json:"redisPubSubNode"`
	RedisPubSubUsername  string   `json:"redisPubSubUsername"`
	RedisPubsubPassword  string   `json:"-"`
	Profile              Profile  `json:"profile"`
	AwsRegion            string   `json:"awsRegion"`
	AwsEndpoint          string   `json:"awsEndpoint"`
	AwsUsername          string   `json:"awsUsername"`
	AwsPassword          string   `json:"-"`
	AllowedOrigins       string   `json:"allowedOrigins"`
	OltpEndpoint         string   `json:"oltpEndpoint"`
	ProfileBucket        string   `json:"profileBucket"`
}

func LoadDotenv() {
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
			slog.Error("invalid line in .env file", "line", line)
			os.Exit(1)
		}
		// these environment variables last until the end of this process
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("error setting env var", "error", err)
		}
	}
}

func loadConfig() Config {
	config := Config{
		ServerPort:           os.Getenv("SERVER_PORT"),
		DbURL:                os.Getenv("DB_URL"),
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
		ProfileBucket:        os.Getenv("PROFILE_BUCKET_NAME"),
		AllowedOrigins:       os.Getenv("ALLOWED_ORIGINS"),
		OltpEndpoint:         os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}

	// logger must be a JSON logger to respect the password commission `json:"-"`
	slog.Info("loaded config", "config", config)

	return config
}

func Load() Config {
	LoadDotenv()
	return loadConfig()
}
