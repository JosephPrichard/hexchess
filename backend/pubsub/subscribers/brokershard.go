package subscribers

import (
	"log/slog"
	"sync"
	"time"
	"uuid"
)

type subscribersMap[Payload any] map[uuid.UUID]*brokerRecord[Payload]

type brokerShard[Payload any] struct {
	lock sync.Mutex

	brokerID       string
	shardID        string
	subscribersMap subscribersMap[Payload]
	pendingRemoval bool

	slowSubscriberTimeout time.Duration
}

func (shard *brokerShard[Payload]) remove(record *brokerRecord[Payload]) {
	delete(shard.subscribersMap, record.id)
	close(record.mailbox)
}

func (shard *brokerShard[Payload]) subscribe(record *brokerRecord[Payload]) {
	subscriberID := record.id

	// `Lock` is done per shard, so broker subscribe operations on different shards can run in parallel
	shard.lock.Lock()

	if !shard.pendingRemoval {
		if shard.subscribersMap == nil {
			shard.subscribersMap = make(subscribersMap[Payload])
		}
		shard.subscribersMap[subscriberID] = record
	} else {
		// edge case: we have a shard that is pending removal (rare), immediately close the subscriber so the client will retry after removal is complete
		close(record.mailbox)
	}

	shard.lock.Unlock()

	slog.Info("PubSub: subscribed to broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriberID)
}

func (shard *brokerShard[Payload]) unsubscribe(subscriberID uuid.UUID) {
	// `Lock` is done per shard, so unsubscribe operations on different shards can run in parallel
	shard.lock.Lock()

	if shard.subscribersMap == nil {
		shard.subscribersMap = make(subscribersMap[Payload])
	}
	if record, recordExists := shard.subscribersMap[subscriberID]; recordExists {
		shard.remove(record)
	}

	shard.lock.Unlock()

	slog.Info("PubSub: unsubscribed from broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriberID)
}

func (shard *brokerShard[Payload]) broadcast(payload Payload) {
	var recvCount, dropCount int

	timeNow := time.Now()

	// `Lock` is done per shard, so broadcast operations on different shards can run in parallel
	shard.lock.Lock()

	if shard.subscribersMap == nil {
		shard.subscribersMap = make(subscribersMap[Payload])
	}
	for _, record := range shard.subscribersMap {
		select {
		// channel `send` to `mailbox` is safe while acquiring a lock since `send` is made non-blocking by the `default` clause
		case record.mailbox <- payload:
			recvCount++
			record.touch()
		// `default` drop a message if the consumer is slow (the buffer is full)
		default:
			dropCount++
			// check if a consumer is slow, and if so - close it permanently
			if shard.slowSubscriberTimeout >= 0 && timeNow.Sub(record.lastUpdated) > shard.slowSubscriberTimeout {
				shard.remove(record)
			}
		}
	}

	shard.lock.Unlock()

	slog.Info("PubSub: broadcasted message to broker shard subscribers",
		"brokerID", shard.brokerID, "shardID", shard.shardID, "recvCount", recvCount, "dropCount", dropCount)
}

func (shard *brokerShard[Payload]) removeIfIdle() bool {
	shard.lock.Lock()

	// a shard is `idle` if it doesn't have any subscribers and should be marked for removal
	isMarkedForRemoval := len(shard.subscribersMap) == 0
	if isMarkedForRemoval {
		shard.subscribersMap = nil
		shard.pendingRemoval = true // marks a shard as pending `removal` so a concurrent subscribe operation will know to reject the next subscriber
	}

	shard.lock.Unlock()
	return isMarkedForRemoval
}
