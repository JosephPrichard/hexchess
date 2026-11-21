package redisub

import (
	"log/slog"
	"sync"
)

type GlobalBroadcaster struct {
	BrokerConfig
	id   string
	lock sync.Mutex
	room *brokerRoom
}

func NewGlobalBroadcaster(brokerID string, config BrokerConfig) *GlobalBroadcaster {
	return &GlobalBroadcaster{
		BrokerConfig: config,
		id:           brokerID,
		room: &brokerRoom{
			idleSubscriberTimeout: config.IdleSubscriberTimeout,
		},
	}
}

func (b *GlobalBroadcaster) Subscribe(capacity int) (<-chan []byte, func()) {
	b.lock.Lock()

	subscriber, subscriberCount := b.room.subscribe(capacity)

	b.lock.Unlock()

	unsubscribe := func() {
		b.unsubscribe(subscriber.id)
	}

	slog.Info("PubSub: subscribed to broker", "brokerID", b.id, "subscriberID", subscriber.id, "subscriberCount", subscriberCount)
	return subscriber.mailbox, unsubscribe
}

func (b *GlobalBroadcaster) unsubscribe(subscriberID SubscriberID) {
	b.lock.Lock()

	remainingSubscribers := b.room.unsubscribe(subscriberID)

	b.lock.Unlock()

	slog.Info("PubSub: unsubscribed from broker", "brokerID", b.id, "subscriberID", subscriberID, "remainingSubscribers", remainingSubscribers)
}

func (b *GlobalBroadcaster) Broadcast(payload []byte) {
	b.lock.Lock()

	recvCount, dropCount := b.room.broadcast(payload)

	b.lock.Unlock()

	slog.Info("PubSub: broadcasted message to broker", "brokerID", b.id, "recvCount", recvCount, "dropCount", dropCount)
}

func (b *GlobalBroadcaster) Close() {}
