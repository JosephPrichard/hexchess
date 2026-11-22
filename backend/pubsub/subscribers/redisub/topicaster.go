package redisub

import (
	"log/slog"
	"sync"
)

type TopicBroadcaster struct {
	BrokerConfig
	id       string
	lock     sync.Mutex
	roomsMap map[string]*brokerRoom
}

func NewTopicBroadcaster(brokerID string, config BrokerConfig) *TopicBroadcaster {
	return &TopicBroadcaster{
		BrokerConfig: config,
		id:           brokerID,
		roomsMap:     make(map[string]*brokerRoom),
	}
}

func (b *TopicBroadcaster) Subscribe(roomID string, capacity int) (<-chan []byte, func()) {
	b.lock.Lock()

	room, roomExists := b.roomsMap[roomID]
	if !roomExists {
		room = &brokerRoom{idleSubscriberTimeout: b.IdleSubscriberTimeout}
		b.roomsMap[roomID] = room
	}

	subscriber, subscriberCount := room.subscribe(capacity)

	b.lock.Unlock()

	unsubscribe := func() {
		b.unsubscribe(roomID, subscriber.id)
	}

	slog.Info("PubSub: subscribed to room", "brokerID", b.id, "roomID", roomID, "subscriberID", subscriber.id, "subscriberCount", subscriberCount)
	return subscriber.mailbox, unsubscribe
}

func (b *TopicBroadcaster) unsubscribe(roomID string, subscriberID SubscriberID) {
	var remainingSubscribers int

	b.lock.Lock()

	room, roomExists := b.roomsMap[roomID]
	if roomExists {
		remainingSubscribers = room.unsubscribe(subscriberID)
		if remainingSubscribers == 0 {
			delete(b.roomsMap, roomID)
		}
	}

	b.lock.Unlock()

	if roomExists {
		slog.Info("PubSub: unsubscribed from room", "brokerID", b.id, "roomID", roomID, "subscriberID", subscriberID, "remainingSubscribers", remainingSubscribers)
	} else {
		slog.Warn("PubSub: unsubscribed from non-existent room", "brokerID", b.id, "roomID", roomID)
	}
}

func (b *TopicBroadcaster) Broadcast(roomID string, payload []byte) {
	var recvCount, dropCount int

	b.lock.Lock()

	room, roomExists := b.roomsMap[roomID]
	if roomExists {
		recvCount, dropCount = room.broadcast(payload)
	}

	b.lock.Unlock()

	if roomExists {
		slog.Info("PubSub: broadcasted message to room", "brokerID", b.id, "roomID", roomID, "recvCount", recvCount, "dropCount", dropCount)
	} else {
		slog.Warn("PubSub: broadcasting to non-existent room", "brokerID", b.id, "roomID", roomID)
	}
}

func (b *TopicBroadcaster) Close() {}
