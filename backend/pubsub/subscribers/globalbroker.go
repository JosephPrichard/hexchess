package subscribers

import (
	"hash/fnv"
	"strconv"
	"uuid"
)

type GlobalBroadcastBroker[Payload any] struct {
	BrokerConfig
	id           string
	brokerShards []*globalBrokerShard[Payload]
}

func NewGlobalBroadcastBroker[Payload any](brokerID string, config BrokerConfig) *GlobalBroadcastBroker[Payload] {
	var brokerShards []*globalBrokerShard[Payload]

	for i := range config.GlobalBrokerShardCount {
		shard := &globalBrokerShard[Payload]{
			brokerID:              brokerID,
			shardID:               strconv.Itoa(i),
			subscribersMap:        make(subscribersMap[Payload]),
			slowSubscriberTimeout: config.SlowSubscriberTimeout,
			workerChan:            make(chan workerJob[Payload]),
		}
		brokerShards = append(brokerShards, shard)
		go runBrokerWorker(shard.workerChan, config.outputChan)
	}

	return &GlobalBroadcastBroker[Payload]{BrokerConfig: config, id: brokerID, brokerShards: brokerShards}
}

func slotSubscriberID(subscriberID uuid.UUID, len int) int {
	h := fnv.New64a()
	h.Write(subscriberID[:])
	hash := h.Sum64()
	return int(hash % uint64(len))
}

func (broker *GlobalBroadcastBroker[Payload]) Subscribe(capacity int) Subscriber[Payload] {
	record := newBrokerRecord[Payload](capacity)
	subscriber := record.subscriber()

	// `slotSubscriberID`: bucket the subscribe call to different shards deterministically
	slot := slotSubscriberID(subscriber.id, len(broker.brokerShards))
	shard := broker.brokerShards[slot]

	shard.sendJob(workerJob[Payload]{kind: subscribeKind, subscribeRecord: record})
	return subscriber
}

func (broker *GlobalBroadcastBroker[Payload]) Unsubscribe(subscriber Subscriber[Payload]) {
	subscriberID := subscriber.id

	// `slotSubscriberID`: bucket unsubscribe call to the same shard as the subscribe call
	slot := slotSubscriberID(subscriberID, len(broker.brokerShards))
	shard := broker.brokerShards[slot]

	shard.sendJob(workerJob[Payload]{kind: unsubscribeKind, unsubscribeID: subscriberID})
}

func (broker *GlobalBroadcastBroker[Payload]) Broadcast(payload Payload) {
	// fan out: every broadcaster needs to get the
	for _, shard := range broker.brokerShards {
		shard.sendJob(workerJob[Payload]{kind: broadcastKind, broadcastPayload: payload})
	}
}

func (broker *GlobalBroadcastBroker[Payload]) Shutdown() {
	for _, shard := range broker.brokerShards {
		close(shard.workerChan)
	}
}

type globalBrokerShard[Payload any] struct {
	brokerShard[Payload]
	workerChan chan workerJob[Payload]
}

func (broker *globalBrokerShard[Payload]) sendJob(job workerJob[Payload]) {
	job.shard = &broker.brokerShard
	broker.workerChan <- job
}
