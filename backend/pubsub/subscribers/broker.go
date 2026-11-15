package subscribers

import (
	"log/slog"
	"sync"
	"time"
)

type BroadcastBroker[Payload any] struct {
	BrokerConfig
	id          string
	shardsMap   sync.Map
	closeFunc   func()
	cleanerDone chan struct{}
	inputChan   chan workerTask[Payload]
}

func NewBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *BroadcastBroker[Payload] {
	broker := &BroadcastBroker[Payload]{
		BrokerConfig: config,
		id:           brokerID,
		shardsMap:    sync.Map{},
		cleanerDone:  make(chan struct{}),
		inputChan:    make(chan workerTask[Payload]),
	}

	broker.closeFunc = sync.OnceFunc(func() {
		close(broker.cleanerDone)
		close(broker.inputChan)
	})

	for range broker.BrokerWorkerCount {
		go runBrokerWorker(broker.inputChan, broker.outputChan)
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

	broker.sendTask(newSubscribeTask(record, shard))

	return subscriber
}

func (broker *BroadcastBroker[Payload]) Unsubscribe(shardID string, subscriber Subscriber[Payload]) {
	subscriberID := subscriber.id

	anyShard, shardExists := broker.shardsMap.Load(shardID)

	if shardExists {
		shard := anyShard.(*brokerShard[Payload])
		broker.sendTask(newUnsubscribeTask(subscriberID, shard))
	}

	if !shardExists {
		slog.Warn("PubSub: unsubscribed to non-existent shard", "brokerID", broker.id, "shardID", shardID)
	}
}

func (broker *BroadcastBroker[Payload]) Broadcast(shardID string, payload Payload) {
	anyShard, shardExists := broker.shardsMap.Load(shardID)

	if shardExists {
		shard := anyShard.(*brokerShard[Payload])
		broker.sendTask(newBroadcastTask(payload, shard))
	}

	if !shardExists {
		slog.Warn("PubSub: broadcasting to non-existent shard", "brokerID", broker.id, "shardID", shardID)
	}
}

func (broker *BroadcastBroker[Payload]) sendTask(task workerTask[Payload]) {
	broker.inputChan <- task
}

func (broker *BroadcastBroker[Payload]) Shutdown() {
	broker.closeFunc()
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

				isRemoved := shard.doWhenEmpty(func() {
					broker.shardsMap.Delete(shardID)
				})
				if isRemoved {
					slog.Info("PubSub: removed empty shard", "brokerID", broker.id, "shardID", shardID)
				}

				return true
			})
		case <-broker.cleanerDone:
			slog.Info("PubSub: shutdown broker cleaner", "brokerID", broker.id)
			return
		}
	}
}
