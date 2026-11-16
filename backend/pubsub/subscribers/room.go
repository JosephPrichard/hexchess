package subscribers

import (
	"time"
	"uuid"
)

type subscribersMap[Payload any] map[SubscriberID]*subscriberRecord[Payload]

type brokerRoom[Payload any] struct {
	subscribersMap        subscribersMap[Payload]
	idleSubscriberTimeout time.Duration
}

func (room *brokerRoom[Payload]) subscribe(capacity int) (Subscriber[Payload], int) {
	record := newBrokerRecord[Payload](capacity)

	subscriberID := record.id

	if room.subscribersMap == nil {
		room.subscribersMap = make(subscribersMap[Payload])
	}
	room.subscribersMap[subscriberID] = record

	return record.toSubscriber(), len(room.subscribersMap)
}

func (room *brokerRoom[Payload]) unsubscribe(subscriberID uuid.UUID) int {
	remainingSubscribers := -1

	if room.subscribersMap != nil {
		if record, recordExists := room.subscribersMap[subscriberID]; recordExists {
			room.remove(record)
			remainingSubscribers = len(room.subscribersMap)
		}
	}

	return remainingSubscribers
}

func (room *brokerRoom[Payload]) broadcast(payload Payload) (int, int) {
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

func (room *brokerRoom[Payload]) remove(record *subscriberRecord[Payload]) {
	delete(room.subscribersMap, record.id)
	close(record.mailbox)
	if len(room.subscribersMap) == 0 {
		room.subscribersMap = nil
	}
}
