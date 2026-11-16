package subscribers

import (
	"log/slog"
	"sync"
	"time"
	"uuid"
)

type subscribersMap[Payload any] map[SubscriberID]*subscriberRecord[Payload]

type brokerShard[Payload any] struct {
	lock sync.Mutex

	brokerID       string
	shardID        string
	subscribersMap subscribersMap[Payload]

	idleSubscriberTimeout time.Duration
}

func (shard *brokerShard[Payload]) subscribe(record *subscriberRecord[Payload]) {
	subscriberID := record.id

	// `Lock` is done per shard, so broker subscribe operations on different shards can run in parallel
	shard.lock.Lock()

	if shard.subscribersMap == nil {
		shard.subscribersMap = make(subscribersMap[Payload])
	}
	shard.subscribersMap[subscriberID] = record

	shard.lock.Unlock()

	slog.Info("PubSub: subscribed to broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriberID)
}

func (shard *brokerShard[Payload]) unsubscribe(subscriberID uuid.UUID) {
	wasRemoved := false

	// `Lock` is done per shard, so unsubscribe operations on different shards can run in parallel
	shard.lock.Lock()

	if shard.subscribersMap != nil {
		if record, recordExists := shard.subscribersMap[subscriberID]; recordExists {
			shard.remove(record)
			wasRemoved = true
		}
	}

	shard.lock.Unlock()

	slog.Info("PubSub: unsubscribed from broker shard",
		"brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriberID, "wasRemoved", wasRemoved)
}

func (shard *brokerShard[Payload]) broadcast(payload Payload) {
	var recvCount, dropCount int

	timeNow := time.Now()

	// `Lock` is done per shard, so broadcast operations on different shards can run in parallel
	shard.lock.Lock()

	if shard.subscribersMap != nil {
		for _, record := range shard.subscribersMap {
			select {
			// channel `send` to `mailbox` is safe while acquiring a lock since `send` is made non-blocking by the `default` clause
			case record.mailbox <- payload:
				recvCount++
				record.touch()
			// `default` drop a message if the consumer is slow (the buffer is full)
			default:
				dropCount++
				// check if a consumer is slow/idle, and if so - close it permanently
				if shard.idleSubscriberTimeout >= 0 && timeNow.Sub(record.lastUpdated) > shard.idleSubscriberTimeout {
					shard.remove(record)
				}
			}
		}
	}

	shard.lock.Unlock()

	slog.Info("PubSub: broadcasted message to broker shard subscribers",
		"brokerID", shard.brokerID, "shardID", shard.shardID, "recvCount", recvCount, "dropCount", dropCount)
}

func (shard *brokerShard[Payload]) remove(record *subscriberRecord[Payload]) {
	delete(shard.subscribersMap, record.id)
	close(record.mailbox)
	if len(shard.subscribersMap) == 0 {
		shard.subscribersMap = nil
	}
}
