package pubsub

import (
	"context"
	"fmt"
	"hexchess-svc/pubsub/publishers/natspub"
	"hexchess-svc/pubsub/publishers/redispub"
	"hexchess-svc/pubsub/subscribers/natsub"
	"hexchess-svc/pubsub/subscribers/redisub"
	"hexchess-svc/utils/slogutil"
	"sync"

	"github.com/nats-io/nats.go"
)

type Publisher interface {
	Publish(ctx context.Context, channel string, topic string, payload []byte)
	Close()
}

type Subscriber interface {
	Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error)
	Close()
}

type PubsubBroker interface {
	Publisher
	Subscriber
	Start(ctx context.Context)
}

func NewPubsubBroker(config PubsubConfig) PubsubBroker {
	if config.Strategy == "" {
		config.Strategy = RedisStrategy
	}
	switch config.Strategy {
	case RedisStrategy:
		return NewRedisPubsubBroker(config)
	case NatsStrategy:
		return NewNatsPubsubBroker(config)
	default:
		panic(fmt.Sprintf("invalid pubsub strategy: %v", config.Strategy))
	}
}

type RedisPubsubBroker struct {
	publisher  *redispub.RedisPublisher
	subscriber *redisub.RedisBroadcastBroker
}

func NewRedisPubsubBroker(config PubsubConfig) *RedisPubsubBroker {
	return &RedisPubsubBroker{
		publisher:  redispub.NewRedisPublisher(config.RedisClient, config.Dispatcher),
		subscriber: redisub.NewRedisBroadcastBroker(redisub.DefaultBrokerConfig, config.RedisClient),
	}
}

func (b *RedisPubsubBroker) Publish(ctx context.Context, channel string, topic string, payload []byte) {
	b.publisher.Publish(ctx, channel, topic, payload)
}

func (b *RedisPubsubBroker) Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error) {
	return b.subscriber.Subscribe(ctx, channel, topic)
}

func (b *RedisPubsubBroker) Start(ctx context.Context) {
	b.subscriber.Start(ctx)
}

func (b *RedisPubsubBroker) Close() {
	b.publisher.Close()
	b.subscriber.Close()
}

type NatsPubsubBroker struct {
	publisher  *natspub.NatsPublisher
	subscriber *natsub.NatsSubscriber
}

func NewNatsPubsubBroker(config PubsubConfig) *NatsPubsubBroker {
	natsConn, err := nats.Connect(config.NatsURI)
	if err != nil {
		slogutil.Fatal("connect to nats", err)
	}
	return &NatsPubsubBroker{
		publisher:  &natspub.NatsPublisher{NatsConn: natsConn},
		subscriber: &natsub.NatsSubscriber{NatsConn: natsConn},
	}
}

func (b *NatsPubsubBroker) Publish(ctx context.Context, channel string, topic string, payload []byte) {
	b.publisher.Publish(ctx, channel, topic, payload)
}

func (b *NatsPubsubBroker) Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error) {
	return b.subscriber.Subscribe(ctx, channel, topic)
}

// Start no-ops because nats is already started
func (b *NatsPubsubBroker) Start(_ context.Context) {}

func (b *NatsPubsubBroker) Close() {
	b.publisher.Close()
	b.subscriber.Close()
}

type Subject struct {
	Channel string
	Topic   string
}

func SubscribeMany(ctx context.Context, subscriber Subscriber, subjects []Subject) (<-chan []byte, func(), error) {
	var chans []<-chan []byte
	var unsubscribeFns []func()

	unsubscribeMany := func() {
		for _, f := range unsubscribeFns {
			f()
		}
	}

	for _, subject := range subjects {
		c, unsubscribe, err := subscriber.Subscribe(ctx, subject.Channel, subject.Topic)
		if err != nil {
			unsubscribeMany()
			return nil, nil, err
		}
		chans = append(chans, c)
		unsubscribeFns = append(unsubscribeFns, unsubscribe)
	}

	var wg sync.WaitGroup
	multiChan := make(chan []byte)

	for _, c := range chans {
		wg.Go(func() {
			for m := range c {
				multiChan <- m
			}
		})
	}

	go func() {
		wg.Wait()
		close(multiChan)
	}()

	return multiChan, unsubscribeMany, nil
}
