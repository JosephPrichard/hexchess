package pubsub

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
)

type BroadcastBroker[ActorID comparable] struct {
	Lock      sync.Mutex
	ID        string
	actorsMap map[ActorID][]chan []byte
}

func NewBroadcastBroker[BrokerID comparable](ID string) *BroadcastBroker[BrokerID] {
	return &BroadcastBroker[BrokerID]{ID: ID, actorsMap: make(map[BrokerID][]chan []byte)}
}

func (broker *BroadcastBroker[ActorID]) Subscribe(actorID ActorID, sub chan []byte) {
	slog.Info("subscribing to broadcaster broker", "actorID", broker.ID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	broker.Lock.Lock()

	shard := broker.actorsMap[actorID]
	if !slices.Contains(shard, sub) {
		shard = append(shard, sub)
	}
	broker.actorsMap[actorID] = shard

	broker.Lock.Unlock()
}

func (broker *BroadcastBroker[ActorID]) Unsubscribe(actorID ActorID, sub chan []byte) {
	slog.Info("unsubscribed from broadcaster broker", "actorID", actorID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	broker.Lock.Lock()

	shard := broker.actorsMap[actorID]
	if slices.Contains(shard, sub) {
		close(sub)
	}
	shard = slices.DeleteFunc(shard, func(subElem chan []byte) bool { return subElem == sub })
	broker.actorsMap[actorID] = shard

	broker.Lock.Unlock()
}

func (broker *BroadcastBroker[ActorID]) Broadcast(actorID ActorID, msg []byte) {
	var subStrs []string

	broker.Lock.Lock()

	shard := broker.actorsMap[actorID]
	if shard != nil {
		for _, sub := range shard {
			select {
			case sub <- msg:
			default:
				// drop a message if the consumer is slow
				slog.Warn("BroadcastActor: dropping broadcasted message", "actorID", actorID)
			}
			subStrs = append(subStrs, fmt.Sprintf("%v", sub))
		}
	}

	broker.Lock.Unlock()

	slog.Info("broadcasted message to broadcaster broker subscribers", "actorID", actorID, "subscribers", subStrs)
}

type CountEventKind int

const (
	GlobalActiveEvent CountEventKind = iota
	GlobalGamesEvent
)

type GlobalCastEvent struct {
	Kind CountEventKind
	Data string
}

type GlobalCasterBroker struct {
	Lock          sync.Mutex
	id            string
	subscriberMap map[chan GlobalCastEvent]struct{}
}

func NewGlobalCasterBroker(id string) *GlobalCasterBroker {
	return &GlobalCasterBroker{id: id, subscriberMap: make(map[chan GlobalCastEvent]struct{})}
}

func (broker *GlobalCasterBroker) Subscribe(sub chan GlobalCastEvent) {
	slog.Info("subscribing to globalcaster", "actorID", broker.id, "sub", fmt.Sprintf("%v", sub))

	broker.Lock.Lock()
	broker.subscriberMap[sub] = struct{}{}
	broker.Lock.Unlock()
}

func (broker *GlobalCasterBroker) Unsubscribe(sub chan GlobalCastEvent) {
	slog.Info("unsubscribing from globalcaster", "actorID", broker.id, "sub", fmt.Sprintf("%v", sub))

	broker.Lock.Lock()
	if _, ok := broker.subscriberMap[sub]; ok {
		delete(broker.subscriberMap, sub)
		close(sub)
	}
	broker.Lock.Unlock()
}

func (broker *GlobalCasterBroker) Broadcast(payload GlobalCastEvent) {
	count := 0

	broker.Lock.Lock()
	for sub := range broker.subscriberMap {
		count++
		select {
		case sub <- payload:
		default:
			// drop a message if the consumer is slow
			slog.Warn("GlobalCasterActor: dropping broadcasted message", "actorID", broker.id)
		}
	}
	broker.Lock.Unlock()

	slog.Info("broadcasted message to globalcaster subscribers", "actorID", broker.id, "count", count)
}
