package redisub

import (
	"time"
	"uuid"
)

type subscribersMap map[SubscriberID]*Subscriber

type brokerRoom struct {
	subscribersMap        subscribersMap
	idleSubscriberTimeout time.Duration
}

func (room *brokerRoom) subscribe(capacity int) (*Subscriber, int) {
	s := newSubscriber(capacity)

	subscriberID := s.id

	if room.subscribersMap == nil {
		room.subscribersMap = make(subscribersMap)
	}
	room.subscribersMap[subscriberID] = s

	return s, len(room.subscribersMap)
}

func (room *brokerRoom) unsubscribe(subscriberID uuid.UUID) int {
	remainingSubscribers := -1

	if room.subscribersMap != nil {
		if record, recordExists := room.subscribersMap[subscriberID]; recordExists {
			room.remove(record)
			remainingSubscribers = len(room.subscribersMap)
		}
	}

	return remainingSubscribers
}

func (room *brokerRoom) broadcast(payload []byte) (int, int) {
	var recvCount, dropCount int

	timeNow := time.Now()

	if room.subscribersMap != nil {
		for _, record := range room.subscribersMap {
			select {
			// channel `send` to `mailbox` is safe while acquiring a lock since `send` is made non-blocking by the `default` clause
			case record.mailbox <- payload:
				recvCount++
				record.lastUpdated = timeNow
			// `default` drop a message if the consumer is slow (the buffer is full)
			default:
				dropCount++
				// check if a consumer is slow/idle, and if so - close it permanently
				if room.idleSubscriberTimeout >= 0 && timeNow.Sub(record.lastUpdated) > room.idleSubscriberTimeout {
					room.remove(record)
				}
			}
		}
	}

	return recvCount, dropCount
}

func (room *brokerRoom) remove(record *Subscriber) {
	delete(room.subscribersMap, record.id)
	close(record.mailbox)
	if len(room.subscribersMap) == 0 {
		room.subscribersMap = nil
	}
}
