package subscribers

import (
	"log/slog"
	"sync"
)

type GlobalBroadcastBroker[Payload any] struct {
	BrokerConfig
	id   string
	lock sync.Mutex
	room *brokerRoom[Payload]
}

func NewGlobalBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *GlobalBroadcastBroker[Payload] {
	return &GlobalBroadcastBroker[Payload]{
		BrokerConfig: config,
		id:           brokerID,
		room: &brokerRoom[Payload]{
			idleSubscriberTimeout: config.IdleSubscriberTimeout,
		},
	}
}

func (broker *GlobalBroadcastBroker[Payload]) Subscribe(capacity int) (Subscriber[Payload], func()) {
	broker.lock.Lock()

	subscriber, subscriberCount := broker.room.subscribe(capacity)

	broker.lock.Unlock()

	unsubscribe := func() {
		broker.unsubscribe(subscriber)
	}

	slog.Info("PubSub: subscribed to broker", "brokerID", broker.id, "subscriberID", subscriber.id, "subscriberCount", subscriberCount)
	return subscriber, unsubscribe
}

func (broker *GlobalBroadcastBroker[Payload]) unsubscribe(subscriber Subscriber[Payload]) {
	broker.lock.Lock()

	remainingSubscribers := broker.room.unsubscribe(subscriber.id)

	broker.lock.Unlock()

	slog.Info("PubSub: unsubscribed from broker", "brokerID", broker.id, "subscriberID", subscriber.id, "remainingSubscribers", remainingSubscribers)
}

func (broker *GlobalBroadcastBroker[Payload]) Broadcast(payload Payload) {
	broker.lock.Lock()

	recvCount, dropCount := broker.room.broadcast(payload)

	broker.lock.Unlock()

	slog.Info("PubSub: broadcasted message to broker", "brokerID", broker.id, "recvCount", recvCount, "dropCount", dropCount)
}

func (broker *GlobalBroadcastBroker[Payload]) Shutdown() {}
