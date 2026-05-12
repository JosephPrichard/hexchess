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

type MulticasterActor struct {
	ID         string
	actionChan chan multicasterAction
	actorsMap  map[string]actorShard
}

type actorShard = []chan []byte

type multicasterAction struct {
	kind    actorActionKind
	actorID string
	sub     chan []byte
	payload []byte
}

func MakeMulticasterActor(ID string) *MulticasterActor {
	multicasterMap := &MulticasterActor{ID: ID, actionChan: make(chan multicasterAction), actorsMap: make(map[string]actorShard)}
	go multicasterMap.Start()
	return multicasterMap
}

func (actor *MulticasterActor) handleSubscription(action multicasterAction) {
	actorID := action.actorID
	sub := action.sub

	slog.Info("subscribing to multicaster actor", "actorID", actor.ID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	shard := actor.actorsMap[actorID]
	if !slices.Contains(shard, sub) {
		shard = append(shard, sub)
	}
	actor.actorsMap[actorID] = shard
}

func (actor *MulticasterActor) handleUnsubscription(action multicasterAction) {
	actorID := action.actorID
	sub := action.sub

	slog.Info("unsubscribed from multicaster actor", "actorID", actorID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	shard := actor.actorsMap[actorID]
	if shard != nil {
		if slices.Contains(shard, sub) {
			close(sub)
		}
		shard = slices.DeleteFunc(shard, func(subElem chan []byte) bool { return subElem == sub })
	}
	actor.actorsMap[actorID] = shard
}

func (actor *MulticasterActor) handleBroadcast(action multicasterAction) {
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

		slog.Info("broadcasted to shardcaster actor subscribers", "actorID", actorID, "subscribers", subStrs, "msg", string(msg))
	}
}

func (actor *MulticasterActor) stop() {
	for _, shard := range actor.actorsMap {
		for _, sub := range shard {
			close(sub)
		}
	}
}

func (actor *MulticasterActor) Start() {
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

	slog.Info("stopping multicaster actor", "actorID", actor.ID)
	actor.stop()
}

func (actor *MulticasterActor) send(action multicasterAction) {
	actor.actionChan <- action
}

func (actor *MulticasterActor) Subscribe(actorID string, sub chan []byte) {
	actor.send(multicasterAction{kind: subAction, actorID: actorID, sub: sub})
}

func (actor *MulticasterActor) Unsubscribe(actorID string, sub chan []byte) {
	actor.send(multicasterAction{kind: unsubAction, actorID: actorID, sub: sub})
}

func (actor *MulticasterActor) Broadcast(actorID string, msg []byte) {
	actor.send(multicasterAction{kind: broadcastAction, actorID: actorID, payload: msg})
}

func (actor *MulticasterActor) Shutdown() {
	close(actor.actionChan)
}

type GlobalEventKind int

const (
	GlobalActiveEvent GlobalEventKind = iota
	GlobalGamesEvent
)

type GlobalCastEvent struct {
	Kind GlobalEventKind
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

func MakeGlobalCasterActor(id string) *GlobalCasterActor {
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

	slog.Info("broadcasted to globalcaster subscribers", "actorID", actor.id, "count", count)
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
