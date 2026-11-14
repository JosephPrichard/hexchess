package subscribers

import (
	"log/slog"
	"sync"
	"time"
	"uuid"
)

const (
	SlowConsumerTimeout = 15 * time.Second
	StaleCheckPeriod    = time.Minute
)

type BroadcastBroker[Payload any] struct {
	lock sync.Mutex

	id         string
	shardsMap  map[string]shardControls[Payload]
	isShutdown bool

	outputChan   chan brokerMessage[string]
	shutdownChan chan struct{}

	slowConsumerTimeout time.Duration
}

type brokerMessageKind int

const (
	removeMessageKind brokerMessageKind = iota
)

type brokerMessage[string comparable] struct {
	kind    brokerMessageKind
	shardID string
}

func NewBroadcastBroker[Payload any](id string) *BroadcastBroker[Payload] {
	broker := &BroadcastBroker[Payload]{
		id:                  id,
		shardsMap:           make(map[string]shardControls[Payload]),
		slowConsumerTimeout: SlowConsumerTimeout,
		outputChan:          make(chan brokerMessage[string]),
		shutdownChan:        make(chan struct{}),
	}
	go runBrokerListener(broker)
	return broker
}

func runBrokerListener[Payload any](broker *BroadcastBroker[Payload]) {
	for message := range broker.outputChan {
		select {
		case <-broker.outputChan:
			switch message.kind {
			case removeMessageKind:
				broker.remove(message.shardID)
				slog.Info("PubSub: removed idle broker shard", "brokerID", broker.id, "shardID", message.shardID)
			default:
				slog.Info("PubSub: skipped unknown broker message", "brokerID", broker.id, "messageKind", message.kind)
			}
		case <-broker.shutdownChan:
			return
		}
	}

	slog.Info("PubSub: shutdown broker listener", "brokerID", broker.id)
}

func (broker *BroadcastBroker[Payload]) set(shardID string, shard shardControls[Payload]) {
	broker.lock.Lock()
	broker.shardsMap[shardID] = shard
	broker.lock.Unlock()
}

func (broker *BroadcastBroker[Payload]) get(shardID string) (shardControls[Payload], bool) {
	broker.lock.Lock()
	shard, exists := broker.shardsMap[shardID]
	broker.lock.Unlock()
	return shard, exists
}

func (broker *BroadcastBroker[Payload]) Subscribe(shardID string, capacity int) Subscriber[Payload] {
	subscriber := newSubscriberRecord[Payload](capacity, broker.slowConsumerTimeout)
	publicSubscriber := subscriber.toPublic()

	shard, exists := broker.get(shardID)

	if !exists {
		shard = newShardControls[Payload](broker.id, shardID, broker.outputChan)
		broker.set(shardID, shard)
	}

	shard.sendSubscribe(subscriber)
	return publicSubscriber
}

func (broker *BroadcastBroker[Payload]) Unsubscribe(shardID string, subscriber Subscriber[Payload]) {
	shard, exists := broker.get(shardID)
	if !exists {
		slog.Warn("PubSub: unsubscribing to non-existent", "brokerID", broker.id, "shardID", shardID)
	}
	shard.sendUnsubscribe(subscriber.id)
}

func (broker *BroadcastBroker[Payload]) Broadcast(shardID string, payload Payload) {
	shard, exists := broker.get(shardID)
	if exists {
		shard.sendBroadcast(payload)
	}
}

func (broker *BroadcastBroker[Payload]) remove(shardID string) {
	broker.lock.Lock()

	shard, exists := broker.shardsMap[shardID]
	if exists {
		delete(broker.shardsMap, shardID)
		shard.close()
	}

	broker.lock.Unlock()
}

func (broker *BroadcastBroker[Payload]) Shutdown() {
	broker.lock.Lock()

	if !broker.isShutdown {
		for shardID, shard := range broker.shardsMap {
			delete(broker.shardsMap, shardID)
			shard.close()
		}

		close(broker.shutdownChan)
		broker.isShutdown = true
	}

	broker.lock.Unlock()

	slog.Info("PubSub: shutdown broker", "brokerID", broker.id)
}

type subscribersMap[Payload any] map[uuid.UUID]*subscriberRecord[Payload]

type shardState[Payload any] struct {
	brokerID       string
	shardID        string
	subscribersMap subscribersMap[Payload]
}

type shardMessageKind int

const (
	broadcastMessageKind shardMessageKind = iota
	subscribeMessageKind
	unsubscribeMessageKind
)

// shardMessage is a "fat struct" implementation of a tagged union that contains all its data and an enumeration indicating which data to use
type shardMessage[Payload any] struct {
	kind shardMessageKind

	broadcastPayload Payload                   // broadcast case
	newSubscriber    subscriberRecord[Payload] // subscribe case
	unsubscribeID    uuid.UUID                 // unsubscribe case
}

type shardControls[Payload any] struct {
	inputChan  chan shardMessage[Payload]
	outputChan chan brokerMessage[string]
}

func newShardControls[Payload any](brokerID string, shardID string, outputChan chan brokerMessage[string]) shardControls[Payload] {
	controls := shardControls[Payload]{inputChan: make(chan shardMessage[Payload]), outputChan: outputChan}
	go runShardListener(brokerID, shardID, controls)
	return controls
}

func (shard *shardControls[Payload]) sendSubscribe(subscriber subscriberRecord[Payload]) {
	shard.inputChan <- shardMessage[Payload]{kind: subscribeMessageKind, newSubscriber: subscriber}
}

func (shard *shardControls[Payload]) sendUnsubscribe(subscriberID uuid.UUID) {
	shard.inputChan <- shardMessage[Payload]{kind: unsubscribeMessageKind, unsubscribeID: subscriberID}
}

func (shard *shardControls[Payload]) sendBroadcast(payload Payload) {
	shard.inputChan <- shardMessage[Payload]{kind: broadcastMessageKind, broadcastPayload: payload}
}

func (shard *shardControls[Payload]) close() {
	close(shard.inputChan)
}

func runShardListener[Payload any](brokerID string, shardID string, shardControls shardControls[Payload]) {
	state := shardState[Payload]{
		brokerID:       brokerID,
		shardID:        shardID,
		subscribersMap: make(subscribersMap[Payload]),
	}

	staleCheckTicker := time.NewTicker(StaleCheckPeriod)
	defer staleCheckTicker.Stop()

RecvLoop:
	for {
		select {
		case message, ok := <-shardControls.inputChan:
			if !ok {
				state.unsubscribeAll()
				break RecvLoop
			}
			switch message.kind {
			case broadcastMessageKind:
				state.broadcast(message.broadcastPayload)
			case subscribeMessageKind:
				state.subscribe(message.newSubscriber)
			case unsubscribeMessageKind:
				state.unsubscribe(message.unsubscribeID)
			default:
				slog.Info("PubSub: skipped unknown shard message", "brokerID", state.brokerID, "shardID", state.shardID, "messageKind", message.kind)
			}
		case <-staleCheckTicker.C:
			// if shard is idle (no subscribers), request removal of shard from broker (owner)
			if state.isEmpty() {
				shardControls.outputChan <- brokerMessage[string]{shardID: state.shardID}
			}
		}
	}

	slog.Info("PubSub: shutdown broker shard", "brokerID", state.brokerID, "shardID", state.shardID)
}

func (shard *shardState[Payload]) isEmpty() bool {
	return len(shard.subscribersMap) == 0
}

func (shard *shardState[Payload]) subscribe(subscriber subscriberRecord[Payload]) {
	slog.Info("PubSub: subscribing to broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriber.id)

	shard.subscribersMap[subscriber.id] = &subscriber // copies to heap and takes ownership
}

func (shard *shardState[Payload]) unsubscribeAll() {
	slog.Info("PubSub: unsubscribing all from broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "count", shard.subscribersMap)

	for _, subscriber := range shard.subscribersMap {
		delete(shard.subscribersMap, subscriber.id)
		close(subscriber.mailbox)
	}
}

func (shard *shardState[Payload]) unsubscribe(subscriberID uuid.UUID) {
	slog.Info("PubSub: unsubscribing from broker shard", "brokerID", shard.brokerID, "shardID", shard.shardID, "subscriberID", subscriberID)

	if subscriber, exists := shard.subscribersMap[subscriberID]; exists {
		delete(shard.subscribersMap, subscriber.id)
		close(subscriber.mailbox)
	}
}

func (shard *shardState[Payload]) broadcast(payload Payload) {
	var successfulSubscribers []uuid.UUID
	var droppedSubscribers []uuid.UUID
	var removeSubscribers []uuid.UUID

	for _, subscriber := range shard.subscribersMap {
		select {
		case subscriber.mailbox <- payload:
			subscriber.touch()
			successfulSubscribers = append(successfulSubscribers, subscriber.id)
		default:
			// drop a message if the consumer is slow
			droppedSubscribers = append(droppedSubscribers, subscriber.id)
			// if messages are continuously dropped, close the slow consumer
			if subscriber.isSlow() {
				removeSubscribers = append(removeSubscribers, subscriber.id)
			}
		}
	}

	for _, subscriberID := range removeSubscribers {
		mailbox := shard.subscribersMap[subscriberID].mailbox
		delete(shard.subscribersMap, subscriberID)
		close(mailbox)
	}

	slog.Info("PubSub: broadcasted message to broadcaster broker subscribers", "brokerID", shard.brokerID, "shardID", shard.shardID,
		"successfulSubscribers", successfulSubscribers, "droppedSubscribers", droppedSubscribers, "dropCount", len(droppedSubscribers))
}
