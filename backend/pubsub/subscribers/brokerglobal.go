package subscribers

import (
	"hash/fnv"
	"log/slog"
	"strconv"
	"sync"
	"uuid"
)

type GlobalBroadcastBroker[Payload any] struct {
	BrokerConfig
	id           string
	brokerShards []*globalBrokerShard[Payload]
	closeFunc    func()
}

func NewGlobalBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *GlobalBroadcastBroker[Payload] {
	var brokerShards []*globalBrokerShard[Payload]

	for i := range config.GlobalBrokerShardCount {
		shard := &globalBrokerShard[Payload]{
			brokerID:              brokerID,
			shardID:               strconv.Itoa(i),
			subscribersMap:        make(subscribersMap[Payload]),
			idleSubscriberTimeout: config.IdleSubscriberTimeout,
			inputChan:             make(chan workerTask[Payload]),
		}
		brokerShards = append(brokerShards, shard)
		go runBrokerWorker(shard.inputChan, config.outputChan)
	}

	return &GlobalBroadcastBroker[Payload]{
		BrokerConfig: config,
		id:           brokerID,
		brokerShards: brokerShards,
		closeFunc: sync.OnceFunc(func() {
			for _, shard := range brokerShards {
				close(shard.inputChan)
			}
		}),
	}
}

func (broker *GlobalBroadcastBroker[Payload]) Subscribe(capacity int) (Subscriber[Payload], func()) {
	record := newBrokerRecord[Payload](capacity)
	subscriber := record.toSubscriber()

	// `slotSubscriberID`: bucket the subscribe call to different shards deterministically
	shardIndex := getSubscriberIDShard(subscriber.id, len(broker.brokerShards))
	shard := broker.brokerShards[shardIndex]

	shard.sendTask(newSubscribeTask(record, nil))

	unsubscribe := func() {
		broker.Unsubscribe(subscriber)
	}
	return subscriber, unsubscribe
}

func (broker *GlobalBroadcastBroker[Payload]) Unsubscribe(subscriber Subscriber[Payload]) {
	subscriberID := subscriber.id

	// `slotSubscriberID`: bucket unsubscribe call to the same shard as the subscribe call
	shardIndex := getSubscriberIDShard(subscriberID, len(broker.brokerShards))
	shard := broker.brokerShards[shardIndex]

	shard.sendTask(newUnsubscribeTask[Payload](subscriberID, nil))
}

func (broker *GlobalBroadcastBroker[Payload]) Broadcast(payload Payload) {
	// fan out: every shard needs to receive the payload (workers can broadcast using available parallelism)
	for _, shard := range broker.brokerShards {
		shard.sendTask(newBroadcastTask(payload, nil))
	}
}

func (broker *GlobalBroadcastBroker[Payload]) Shutdown() {
	broker.closeFunc()
	slog.Info("PubSub: shutdown broker", "brokerID", broker.id)
}

type globalBrokerShard[Payload any] struct {
	brokerShard[Payload]
	inputChan chan workerTask[Payload]
}

func (shard *globalBrokerShard[Payload]) sendTask(task workerTask[Payload]) {
	task.shard = &shard.brokerShard
	shard.inputChan <- task
}

func getSubscriberIDShard(subscriberID uuid.UUID, len int) int {
	h := fnv.New64a()
	h.Write(subscriberID[:])
	hash := h.Sum64()
	return int(hash % uint64(len))
}
