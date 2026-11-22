package natsub

import (
	"context"
	"fmt"
	"hexchess-svc/pubsub/event"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type NatsSubscriber struct {
	NatsConn *nats.Conn
}

func (s *NatsSubscriber) Subscribe(ctx context.Context, channel string, topic string) (<-chan []byte, func(), error) {
	c := make(chan []byte, event.ChannelBufSize)

	subj := fmt.Sprintf("%s:%s", channel, topic)
	slog.InfoContext(ctx, "PubSub: subscribing to nats subject", "subj", subj)

	sub, err := s.NatsConn.Subscribe(subj, func(m *nats.Msg) {
		select {
		case c <- m.Data:
		default:
			slog.WarnContext(ctx, "PubSub: dropped a received nats message", "subj", subj)
		}
	})
	if err != nil {
		return nil, nil, err
	}

	unsubscribe := func() {
		if err := sub.Unsubscribe(); err != nil {
			slog.ErrorContext(ctx, "PubSub: failed to unsubscribe from nats", "error", err)
		}
		close(c)
	}
	return c, unsubscribe, nil
}

func (s *NatsSubscriber) Close() {
	s.NatsConn.Close()
}
