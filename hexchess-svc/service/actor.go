package svc

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
	stopAction
)

type MulticasterActor struct {
	ID         string
	actionChan chan multicasterAction
}

type multicasterAction struct {
	kind    actorActionKind
	actorID string
	sub     chan []byte
	payload []byte
}

func MakeMulticasterActor(ID string) *MulticasterActor {
	multicasterMap := &MulticasterActor{ID: ID, actionChan: make(chan multicasterAction)}
	go multicasterMap.Start()
	return multicasterMap
}

func (actor *MulticasterActor) Start() {
	type actorShard = []chan []byte
	actorsMap := make(map[string]actorShard)

	handleSubscription := func(action multicasterAction) {
		actorID := action.actorID
		sub := action.sub

		slog.Info("subscribing to multicaster actor", "actorID", actor.ID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

		shard := actorsMap[actorID]
		if !slices.Contains(shard, sub) {
			shard = append(shard, sub)
		}
		actorsMap[actorID] = shard
	}

	handleUnsubscription := func(action multicasterAction) {
		actorID := action.actorID
		sub := action.sub

		slog.Info("unsubscribed from multicaster actor", "actorID", actorID, "shardActorID", actorID, "sub", fmt.Sprintf("%v", sub))

		shard := actorsMap[actorID]
		if shard != nil {
			if slices.Contains(shard, sub) {
				close(sub)
			}
			shard = slices.DeleteFunc(shard, func(subElem chan []byte) bool { return subElem == sub })
		}
		actorsMap[actorID] = shard
	}

	handleBroadcast := func(action multicasterAction) {
		actorID := action.actorID
		msg := action.payload

		shard := actorsMap[actorID]
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

	handleStop := func() {
		slog.Info("stopping multicaster actor", "actorID", actor.ID)

		for _, shard := range actorsMap {
			for _, sub := range shard {
				close(sub)
			}
		}
	}

	for action := range actor.actionChan {
		switch action.kind {
		case subAction:
			handleSubscription(action)
		case unsubAction:
			handleUnsubscription(action)
		case broadcastAction:
			handleBroadcast(action)
		case stopAction:
			handleStop()
			return
		}
	}
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

type UcEventKind int

const (
	UcActiveEvent UcEventKind = iota
	UcGamesEvent
)

type UcEvent struct {
	Kind UcEventKind
	Data string
}

type GlobalCasterActor struct {
	id         string
	actionChan chan globalcasterAction
}

type globalcasterAction struct {
	action  actorActionKind
	sub     chan UcEvent
	payload UcEvent
}

func (actor GlobalCasterActor) Run() {
	subscriberMap := make(map[chan UcEvent]struct{})

	handleSubscription := func(action globalcasterAction) {
		sub := action.sub

		slog.Info("subscribing to globalcaster", "actorID", actor.id, "sub", fmt.Sprintf("%v", sub))

		subscriberMap[sub] = struct{}{}
	}

	handleUnsubscription := func(action globalcasterAction) {
		sub := action.sub

		slog.Info("unsubscribing from globalcaster", "actorID", actor.id, "sub", fmt.Sprintf("%v", sub))

		if _, ok := subscriberMap[sub]; ok {
			delete(subscriberMap, sub)
			close(sub)
		}
	}

	handleBroadcast := func(action globalcasterAction) {
		msg := action.payload

		count := 0

		for sub := range subscriberMap {
			count++
			select {
			case sub <- msg:
			default:
				// drop a message if the consumer is slow
			}
		}

		slog.Info("broadcasted to globalcaster subscribers", "actorID", actor.id, "count", count)
	}

	handleStop := func() {
		slog.Info("stopping globalcaster", "actorID", actor.id)

		for sub := range subscriberMap {
			close(sub)
		}
	}

	for action := range actor.actionChan {
		switch action.action {
		case subAction:
			handleSubscription(action)
		case unsubAction:
			handleUnsubscription(action)
		case broadcastAction:
			handleBroadcast(action)
		case stopAction:
			handleStop()
			return
		}
	}
}

func MakeGlobalCasterActor(id string) *GlobalCasterActor {
	actor := &GlobalCasterActor{id: id, actionChan: make(chan globalcasterAction)}
	go actor.Run()
	return actor
}

func (actor GlobalCasterActor) send(action globalcasterAction) {
	actor.actionChan <- action
}

func (actor GlobalCasterActor) Subscribe(sub chan UcEvent) {
	actor.send(globalcasterAction{action: subAction, sub: sub})
}

func (actor GlobalCasterActor) Unsubscribe(sub chan UcEvent) {
	actor.send(globalcasterAction{action: unsubAction, sub: sub})
}

func (actor GlobalCasterActor) Broadcast(msg UcEvent) {
	actor.send(globalcasterAction{action: broadcastAction, payload: msg})
}

func (actor GlobalCasterActor) Shutdown() {
	close(actor.actionChan)
}
