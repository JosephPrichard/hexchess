package redispub

import (
	"context"
	"hexchess-svc/utils/goutil"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	client     *redis.Client
	dispatcher goutil.Dispatcher
}

func NewRedisPublisher(client *redis.Client, dispatcher goutil.Dispatcher) *RedisPublisher {
	if client == nil {
		panic("PubSub: while initializing publisher, redis client was nil")
	}
	if dispatcher == nil {
		dispatcher = goutil.SyncDispatcher{}
	}
	return &RedisPublisher{
		client:     client,
		dispatcher: dispatcher,
	}
}

func (p *RedisPublisher) Publish(ctx context.Context, channel string, _ string, payload []byte) {
	ctx = context.WithoutCancel(ctx)
	p.dispatcher.Go(func() {
		recvCount, err := p.client.Publish(ctx, channel, string(payload)).Result()
		if err != nil {
			slog.ErrorContext(ctx, "PubSub: failed to publish message to channel", "channel", channel, "error", err)
		} else {
			slog.InfoContext(ctx, "PubSub: published message to channel",
				"channel", channel, "recvCount", recvCount, "connURL", p.client.String())
		}
	})
}

func (p *RedisPublisher) Close() {
	p.client.Close()
}
