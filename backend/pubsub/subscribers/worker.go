package subscribers

import (
	"log/slog"
)

type BrokerRequestKind int

const (
	subscribeKind BrokerRequestKind = iota
	unsubscribeKind
	broadcastKind
)

type workerTask[Payload any] struct {
	kind BrokerRequestKind // the kind of operation to perform

	shard *brokerShard[Payload] // shard to perform the operation on

	// data associated with the kind of operation
	subscribeRecord  *subscriberRecord[Payload] // subscribe requests
	unsubscribeID    SubscriberID               // unsubscribe requests
	broadcastPayload Payload                    // broadcast requests
}

func newSubscribeTask[Payload any](subscribeRecord *subscriberRecord[Payload], shard *brokerShard[Payload]) workerTask[Payload] {
	return workerTask[Payload]{kind: subscribeKind, subscribeRecord: subscribeRecord, shard: shard}
}

func newUnsubscribeTask[Payload any](unsubscribeID SubscriberID, shard *brokerShard[Payload]) workerTask[Payload] {
	return workerTask[Payload]{kind: unsubscribeKind, unsubscribeID: unsubscribeID, shard: shard}
}

func newBroadcastTask[Payload any](broadcastPayload Payload, shard *brokerShard[Payload]) workerTask[Payload] {
	return workerTask[Payload]{kind: broadcastKind, broadcastPayload: broadcastPayload, shard: shard}
}

func runBrokerWorker[Payload any](inputChan chan workerTask[Payload], outputChan chan struct{}) {
	for request := range inputChan {
		shard := request.shard

		switch request.kind {
		case subscribeKind:
			shard.subscribe(request.subscribeRecord)
		case unsubscribeKind:
			shard.unsubscribe(request.unsubscribeID)
		case broadcastKind:
			shard.broadcast(request.broadcastPayload)
		default:
			slog.Warn("PubSub: worker received unknown request kind", "brokerID", shard.brokerID, "shardID", shard.shardID, "kind", request.kind)
		}

		if outputChan != nil {
			outputChan <- struct{}{}
		}
	}
}
