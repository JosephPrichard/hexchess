package subscribers

import (
	"log/slog"
	"uuid"
)

type BrokerRequestKind int

const (
	subscribeKind BrokerRequestKind = iota
	unsubscribeKind
	broadcastKind
)

type workerJob[Payload any] struct {
	kind BrokerRequestKind // the kind of operation to perform

	shard *brokerShard[Payload] // shard to perform the operation on

	// data associated with the kind of operation
	subscribeRecord  *brokerRecord[Payload] // subscribe requests
	unsubscribeID    uuid.UUID              // unsubscribe requests
	broadcastPayload Payload                // broadcast requests
}

func runBrokerWorker[Payload any](inputChan chan workerJob[Payload], outputChan chan struct{}) {
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
