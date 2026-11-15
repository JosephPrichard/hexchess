package subscribers

import (
	"log/slog"
	"runtime"
	"sync"
	"time"
)

type BrokerConfig struct {
	BrokerWorkerCount      int
	GlobalBrokerShardCount int
	SlowSubscriberTimeout  time.Duration
	StaleCheckPeriod       time.Duration
	outputChan             chan struct{}
}

var DefaultBrokerConfig = BrokerConfig{
	BrokerWorkerCount:      runtime.GOMAXPROCS(0),
	GlobalBrokerShardCount: runtime.GOMAXPROCS(0),
	SlowSubscriberTimeout:  15 * time.Second,
	StaleCheckPeriod:       time.Minute,
}

var MinimalBrokerConfig = BrokerConfig{
	BrokerWorkerCount:      1,
	GlobalBrokerShardCount: 1,
	SlowSubscriberTimeout:  -1,
	StaleCheckPeriod:       -1,
}

type BroadcastBroker[Payload any] struct {
	BrokerConfig
	id          string
	shardsMap   sync.Map
	closeFunc   func()
	cleanerDone chan struct{}
	workerChan  chan workerJob[Payload]
}

func NewBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *BroadcastBroker[Payload] {
	broker := &BroadcastBroker[Payload]{
		BrokerConfig: config,
		id:           brokerID,
		shardsMap:    sync.Map{},
		cleanerDone:  make(chan struct{}),
		workerChan:   make(chan workerJob[Payload]),
	}

	for range broker.BrokerWorkerCount {
		go runBrokerWorker(broker.workerChan, broker.outputChan)
	}

	if broker.StaleCheckPeriod >= 0 {
		go broker.cleanIdleShards()
	}
	return broker
}

func (broker *BroadcastBroker[Payload]) Subscribe(shardID string, capacity int) Subscriber[Payload] {
	record := newBrokerRecord[Payload](capacity)
	subscriber := record.subscriber()

	// `LoadOrStore` guarantees that two concurrent subscribe operations get the same shard
	newShard := &brokerShard[Payload]{
		brokerID:              broker.id,
		shardID:               shardID,
		slowSubscriberTimeout: broker.SlowSubscriberTimeout,
	}
	anyShard, _ := broker.shardsMap.LoadOrStore(shardID, newShard)
	shard := anyShard.(*brokerShard[Payload])

	broker.sendJob(workerJob[Payload]{kind: subscribeKind, shard: shard, subscribeRecord: record})

	return subscriber
}

func (broker *BroadcastBroker[Payload]) Unsubscribe(shardID string, subscriber Subscriber[Payload]) {
	subscriberID := subscriber.id

	anyShard, shardExists := broker.shardsMap.Load(shardID)

	if shardExists {
		shard := anyShard.(*brokerShard[Payload])
		broker.sendJob(workerJob[Payload]{kind: unsubscribeKind, shard: shard, unsubscribeID: subscriberID})
	}

	if !shardExists {
		slog.Warn("PubSub: unsubscribed to non-existent shard", "brokerID", broker.id, "shardID", shardID)
	}
}

func (broker *BroadcastBroker[Payload]) Broadcast(shardID string, payload Payload) {
	anyShard, shardExists := broker.shardsMap.Load(shardID)

	if shardExists {
		shard := anyShard.(*brokerShard[Payload])
		broker.sendJob(workerJob[Payload]{kind: broadcastKind, shard: shard, broadcastPayload: payload})
	}

	if !shardExists {
		slog.Warn("PubSub: broadcasting to non-existent shard", "brokerID", broker.id, "shardID", shardID)
	}
}

func (broker *BroadcastBroker[Payload]) sendJob(job workerJob[Payload]) {
	broker.workerChan <- job
}

func (broker *BroadcastBroker[Payload]) Shutdown() {
	close(broker.cleanerDone)
	close(broker.workerChan)
	slog.Info("PubSub: shutdown broker", "brokerID", broker.id)
}

func (broker *BroadcastBroker[Payload]) cleanIdleShards() {
	ticker := time.NewTicker(broker.StaleCheckPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			broker.shardsMap.Range(func(shardID, anyShard any) bool {
				shard := anyShard.(*brokerShard[Payload])

				isMarkedForRemoval := shard.removeIfIdle()
				if isMarkedForRemoval {
					// removing the `idle` shard from the broker must be done outside the shard's lock
					broker.shardsMap.Delete(shardID)
					slog.Info("PubSub: removed idle shard", "brokerID", broker.id, "shardID", shardID)
				}
				return true
			})
		case <-broker.cleanerDone:
			slog.Info("PubSub: shutdown broker cleaner", "brokerID", broker.id)
			return
		}
	}
}
