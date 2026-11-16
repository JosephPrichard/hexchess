package subscribers

import (
	"log/slog"
	"sync"
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
	return broker
}

func (broker *BroadcastBroker[Payload]) Subscribe(shardID string, capacity int) (Subscriber[Payload], func()) {
	record := newBrokerRecord[Payload](capacity)
	subscriber := record.toSubscriber()

	// `LoadOrStore` guarantees that two concurrent subscribe operations get the same shard
	newShard := &brokerShard[Payload]{
		brokerID:              broker.id,
		shardID:               shardID,
		idleSubscriberTimeout: broker.IdleSubscriberTimeout,
	}
	anyShard, _ := broker.shardsMap.LoadOrStore(shardID, newShard)
	shard := anyShard.(*brokerShard[Payload])

	broker.sendTask(newSubscribeTask(record, shard))

	unsubscribe := func() {
		broker.Unsubscribe(shardID, subscriber)
	}
	return subscriber, unsubscribe
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
