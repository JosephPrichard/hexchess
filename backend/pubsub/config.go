package pubsub

import (
	"hexchess-svc/utils/goutil"

	"github.com/redis/go-redis/v9"
)

type PubsubConfig struct {
	Strategy    string
	NatsURI     string
	RedisClient *redis.Client
	Dispatcher  goutil.Dispatcher
}

const (
	RedisStrategy = "REDIS"
	NatsStrategy  = "NATS"
)
