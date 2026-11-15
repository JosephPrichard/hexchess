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

	slowSubscriberTimeout time.Duration
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

func (shard *brokerShard[Payload]) doWhenEmpty(removeFn func()) bool {
	shard.lock.Lock()

	// a shard is `empty` if it doesn't have any subscribers and should be removed (to save memory)
	isRemoved := len(shard.subscribersMap) == 0
	if isRemoved {
		shard.subscribersMap = nil
		// callback to remove the shard from its owning data-structure (this function *CANNOT* acquire another lock on this shard)
		// executing callback in lock ensures that checking subscribers is atomic with regard to removal - subscribersMap is guaranteed to stay empty
		removeFn()
	}

	shard.lock.Unlock()

	return isRemoved
}

func (shard *brokerShard[Payload]) remove(record *subscriberRecord[Payload]) {
	delete(shard.subscribersMap, record.id)
	close(record.mailbox)
}
