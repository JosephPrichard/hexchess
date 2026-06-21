package pubsub

import (
	"fmt"
	"log/slog"
	"slices"
)

type actorActionKind int

const (
	subAction actorActionKind = iota
	unsubAction
	broadcastAction
)

type BroadcastActor[ActorID comparable] struct {
	ID         string
	actionChan chan broadcasterAction[ActorID]
	actorsMap  map[ActorID][]chan []byte
}

type broadcasterAction[ActorID comparable] struct {
	kind    actorActionKind
	actorID ActorID
	sub     chan []byte
	payload []byte
}

func NewBroadcastActor[ActorID comparable](ID string) *BroadcastActor[ActorID] {
	actor := &BroadcastActor[ActorID]{ID: ID, actionChan: make(chan broadcasterAction[ActorID]), actorsMap: make(map[ActorID][]chan []byte)}
	go actor.Start()
	return actor
}

func (actor *BroadcastActor[ActorID]) handleSubscription(action broadcasterAction[ActorID]) {
	actorID := action.actorID
	sub := action.sub

	slog.Info("subscribing to broadcaster actor", "actorID", actor.ID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	shard := actor.actorsMap[actorID]
	if !slices.Contains(shard, sub) {
		shard = append(shard, sub)
	}
	actor.actorsMap[actorID] = shard
}

func (actor *BroadcastActor[ActorID]) handleUnsubscription(action broadcasterAction[ActorID]) {
	actorID := action.actorID
	sub := action.sub

	slog.Info("unsubscribed from broadcaster actor", "actorID", actorID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	shard := actor.actorsMap[actorID]
	if slices.Contains(shard, sub) {
		close(sub)
	}
	shard = slices.DeleteFunc(shard, func(subElem chan []byte) bool { return subElem == sub })
	actor.actorsMap[actorID] = shard
}

func (actor *BroadcastActor[ActorID]) handleBroadcast(action broadcasterAction[ActorID]) {
	actorID := action.actorID
	msg := action.payload

	shard := actor.actorsMap[actorID]
	if shard != nil {
		var subStrs []string

		for _, sub := range shard {
			select {
			case sub <- msg:
			default:
				// drop a message if the consumer is slow
			}
			subStrs = append(subStrs, fmt.Sprintf("%v", sub))
		}

		slog.Info("broadcasted message to broadcaster actor subscribers", "actorID", actorID, "subscribers", subStrs)
	}
}

func (actor *BroadcastActor[ActorID]) stop() {
	for _, shard := range actor.actorsMap {
		for _, sub := range shard {
			close(sub)
		}
	}
}

func (actor *BroadcastActor[ActorID]) Start() {
	for action := range actor.actionChan {
		switch action.kind {
		case subAction:
			actor.handleSubscription(action)
		case unsubAction:
			actor.handleUnsubscription(action)
		case broadcastAction:
			actor.handleBroadcast(action)
		}
	}

	slog.Info("stopping broadcaster actor", "actorID", actor.ID)
	actor.stop()
}

func (actor *BroadcastActor[ActorID]) send(action broadcasterAction[ActorID]) {
	actor.actionChan <- action
}

func (actor *BroadcastActor[ActorID]) Subscribe(actorID ActorID, sub chan []byte) {
	actor.send(broadcasterAction[ActorID]{kind: subAction, actorID: actorID, sub: sub})
}

func (actor *BroadcastActor[ActorID]) Unsubscribe(actorID ActorID, sub chan []byte) {
	actor.send(broadcasterAction[ActorID]{kind: unsubAction, actorID: actorID, sub: sub})
}

func (actor *BroadcastActor[ActorID]) Broadcast(actorID ActorID, msg []byte) {
	actor.send(broadcasterAction[ActorID]{kind: broadcastAction, actorID: actorID, payload: msg})
}

func (actor *BroadcastActor[ActorID]) Shutdown() {
	close(actor.actionChan)
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

type GlobalCasterActor struct {
	id            string
	actionChan    chan globalcasterAction
	subscriberMap map[chan GlobalCastEvent]struct{}
}

type globalcasterAction struct {
	kind    actorActionKind
	sub     chan GlobalCastEvent
	payload GlobalCastEvent
}

func NewGlobalCasterActor(id string) *GlobalCasterActor {
	actor := &GlobalCasterActor{id: id, actionChan: make(chan globalcasterAction), subscriberMap: make(map[chan GlobalCastEvent]struct{})}
	go actor.Start()
	return actor
}

func (actor *GlobalCasterActor) handleSubscription(action globalcasterAction) {
	sub := action.sub

	slog.Info("subscribing to globalcaster", "actorID", actor.id, "sub", fmt.Sprintf("%v", sub))

	actor.subscriberMap[sub] = struct{}{}
}

func (actor *GlobalCasterActor) handleUnsubscription(action globalcasterAction) {
	sub := action.sub

	slog.Info("unsubscribing from globalcaster", "actorID", actor.id, "sub", fmt.Sprintf("%v", sub))

	if _, ok := actor.subscriberMap[sub]; ok {
		delete(actor.subscriberMap, sub)
		close(sub)
	}
}

func (actor *GlobalCasterActor) handleBroadcast(action globalcasterAction) {
	msg := action.payload

	count := 0

	for sub := range actor.subscriberMap {
		count++
		select {
		case sub <- msg:
		default:
			// drop a message if the consumer is slow
		}
	}

	slog.Info("broadcasted message to globalcaster subscribers", "actorID", actor.id, "count", count)
}

func (actor *GlobalCasterActor) stop() {
	for sub := range actor.subscriberMap {
		close(sub)
	}
}

func (actor GlobalCasterActor) Start() {
	for action := range actor.actionChan {
		switch action.kind {
		case subAction:
			actor.handleSubscription(action)
		case unsubAction:
			actor.handleUnsubscription(action)
		case broadcastAction:
			actor.handleBroadcast(action)
		}
	}

	slog.Info("stopping globalcaster", "actorID", actor.id)

	actor.stop()
}

func (actor GlobalCasterActor) send(action globalcasterAction) {
	actor.actionChan <- action
}

func (actor GlobalCasterActor) Subscribe(sub chan GlobalCastEvent) {
	actor.send(globalcasterAction{kind: subAction, sub: sub})
}

func (actor GlobalCasterActor) Unsubscribe(sub chan GlobalCastEvent) {
	actor.send(globalcasterAction{kind: unsubAction, sub: sub})
}

func (actor GlobalCasterActor) Broadcast(msg GlobalCastEvent) {
	actor.send(globalcasterAction{kind: broadcastAction, payload: msg})
}

func (actor GlobalCasterActor) Shutdown() {
	close(actor.actionChan)
}
