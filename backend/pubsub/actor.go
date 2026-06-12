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

type BroadcastActor struct {
	ID         string
	actionChan chan broadcasterAction
	actorsMap  map[string][]chan []byte
}

type broadcasterAction struct {
	kind    actorActionKind
	actorID string
	sub     chan []byte
	payload []byte
}

func MakeBroadcastActor(ID string) *BroadcastActor {
	actor := &BroadcastActor{ID: ID, actionChan: make(chan broadcasterAction), actorsMap: make(map[string][]chan []byte)}
	go actor.Start()
	return actor
}

func (actor *BroadcastActor) handleSubscription(action broadcasterAction) {
	actorID := action.actorID
	sub := action.sub

	slog.Info("subscribing to broadcaster actor", "actorID", actor.ID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

	shard := actor.actorsMap[actorID]
	if !slices.Contains(shard, sub) {
		shard = append(shard, sub)
	}
	actor.actorsMap[actorID] = shard
}

func (actor *BroadcastActor) handleUnsubscription(action broadcasterAction) {
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

func (actor *BroadcastActor) handleBroadcast(action broadcasterAction) {
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

func (actor *BroadcastActor) stop() {
	for _, shard := range actor.actorsMap {
		for _, sub := range shard {
			close(sub)
		}
	}
}

func (actor *BroadcastActor) Start() {
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

func (actor *BroadcastActor) send(action broadcasterAction) {
	actor.actionChan <- action
}

func (actor *BroadcastActor) Subscribe(actorID string, sub chan []byte) {
	actor.send(broadcasterAction{kind: subAction, actorID: actorID, sub: sub})
}

func (actor *BroadcastActor) Unsubscribe(actorID string, sub chan []byte) {
	actor.send(broadcasterAction{kind: unsubAction, actorID: actorID, sub: sub})
}

func (actor *BroadcastActor) Broadcast(actorID string, msg []byte) {
	actor.send(broadcasterAction{kind: broadcastAction, actorID: actorID, payload: msg})
}

func (actor *BroadcastActor) Shutdown() {
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
