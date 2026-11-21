package pubsub

import (
	"context"
	"fmt"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/pubsub/publishers/redispub"
	"hexchess-svc/pubsub/subscribers/redisub"
	"hexchess-svc/utils/slogutil"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type Publisher interface {
	Publish(ctx context.Context, channel string, topic string, payload []byte)
}

type Subscriber interface {
	Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error)
}

type PubsubBroker interface {
	Publisher
	Subscriber
	Start(ctx context.Context)
	Close()
}

func NewPubsubBroker(config PubsubConfig) PubsubBroker {
	if config.Strategy == "" {
		config.Strategy = RedisStrategy
	}
	switch config.Strategy {
	case RedisStrategy:
		return &RedisPubsubBroker{
			RedisPublisher:       redispub.NewRedisPublisher(config.RedisClient, config.Dispatcher),
			RedisBroadcastBroker: redisub.NewRedisBroadcastBroker(redisub.DefaultBrokerConfig, config.RedisClient),
		}
	case NatsStrategy:
		nc, err := nats.Connect(config.NatsURI)
		if err != nil {
			slogutil.Fatal("connect to nats", err)
		}
		return &NatsPubsubBroker{Conn: nc}
	default:
		panic(fmt.Sprintf("invalid pubsub strategy: %v", config.Strategy))
	}
}

type RedisPubsubBroker struct {
	*redispub.RedisPublisher
	*redisub.RedisBroadcastBroker
}

type NatsPubsubBroker struct {
	*nats.Conn
}

func (b *NatsPubsubBroker) Publish(ctx context.Context, channel string, topic string, payload []byte) {
	subj := fmt.Sprintf("%s:%s", channel, topic)
	slog.InfoContext(ctx, "Pubsub: publishing to nats subject", "subj", subj)

	if err := b.Conn.Publish(subj, payload); err != nil {
		slog.ErrorContext(ctx, "Pubsub: failed to publish to nats", "error", err)
	}
}

func (b *NatsPubsubBroker) Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error) {
	targetChan := make(chan *nats.Msg, event.ChannelBufSize)

	subj := fmt.Sprintf("%s:%s", channel, topic)
	slog.InfoContext(ctx, "Pubsub: subscribing to nats subject", "subj", subj)

	sub, err := b.Conn.ChanSubscribe(subj, targetChan)
	if err != nil {
		return nil, nil, err
	}

	outputChan := make(chan []byte)
	go func() {
		for msg := range targetChan {
			outputChan <- msg.Data
		}
	}()
	unsubscribe := func() {
		if err := sub.Unsubscribe(); err != nil {
			slog.ErrorContext(ctx, "Pubsub: failed to unsubscribe from nats", "error", err)
		}
		close(targetChan)
	}

	return outputChan, unsubscribe, nil
}

// Start no-ops because nats is already started
func (b *NatsPubsubBroker) Start(_ context.Context) {}
