package subscribers

import (
	"log/slog"
	"sync"
)

type BroadcastBroker[Payload any] struct {
	BrokerConfig
	id       string
	lock     sync.Mutex
	roomsMap map[string]*brokerRoom[Payload]
}

func NewBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *BroadcastBroker[Payload] {
	return &BroadcastBroker[Payload]{
		BrokerConfig: config,
		id:           brokerID,
		roomsMap:     make(map[string]*brokerRoom[Payload]),
	}
}

func (broker *BroadcastBroker[Payload]) Subscribe(roomID string, capacity int) (Subscriber[Payload], func()) {
	broker.lock.Lock()

	room, roomExists := broker.roomsMap[roomID]
	if !roomExists {
		room = &brokerRoom[Payload]{idleSubscriberTimeout: broker.IdleSubscriberTimeout}
		broker.roomsMap[roomID] = room
	}

	subscriber, subscriberCount := room.subscribe(capacity)

	broker.lock.Unlock()

	unsubscribe := func() {
		broker.unsubscribe(roomID, subscriber)
	}

	slog.Info("PubSub: subscribed to room", "brokerID", broker.id, "roomID", roomID, "subscriberID", subscriber.id, "subscriberCount", subscriberCount)
	return subscriber, unsubscribe
}

func (broker *BroadcastBroker[Payload]) unsubscribe(roomID string, subscriber Subscriber[Payload]) {
	var remainingSubscribers int

	broker.lock.Lock()

	room, roomExists := broker.roomsMap[roomID]
	if roomExists {
		remainingSubscribers = room.unsubscribe(subscriber.id)
		if remainingSubscribers == 0 {
			delete(broker.roomsMap, roomID)
		}
	}

	broker.lock.Unlock()

	if roomExists {
		slog.Info("PubSub: unsubscribed from room", "brokerID", broker.id, "roomID", roomID, "subscriberID", subscriber.id, "remainingSubscribers", remainingSubscribers)
	} else {
		slog.Warn("PubSub: unsubscribed from non-existent room", "brokerID", broker.id, "roomID", roomID)
	}
}

func (broker *BroadcastBroker[Payload]) Broadcast(roomID string, payload Payload) {
	var recvCount, dropCount int

	broker.lock.Lock()

	room, roomExists := broker.roomsMap[roomID]
	if roomExists {
		recvCount, dropCount = room.broadcast(payload)
	}

	broker.lock.Unlock()

	if roomExists {
		slog.Info("PubSub: broadcasted message to room", "brokerID", broker.id, "roomID", roomID, "recvCount", recvCount, "dropCount", dropCount)
	} else {
		slog.Warn("PubSub: broadcasting to non-existent room", "brokerID", broker.id, "roomID", roomID)
	}
}

func (broker *BroadcastBroker[Payload]) Shutdown() {}
