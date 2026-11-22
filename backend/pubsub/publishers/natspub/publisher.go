package natspub

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type NatsPublisher struct {
	NatsConn *nats.Conn
}

func (p *NatsPublisher) Publish(ctx context.Context, channel string, topic string, payload []byte) {
	subj := fmt.Sprintf("%s:%s", channel, topic)

	err := p.NatsConn.Publish(subj, payload)
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to publish to nats subject", "subj", subj, "error", err)
	} else {
		slog.InfoContext(ctx, "PubSub: published to nats subject", "subj", subj)
	}
}

func (p *NatsPublisher) Close() {
	p.NatsConn.Close()
}
