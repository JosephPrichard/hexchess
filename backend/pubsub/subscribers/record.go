package subscribers

import (
	"time"
	"uuid"
)

type brokerRecord[Payload any] struct {
	id          uuid.UUID
	mailbox     chan Payload
	lastUpdated time.Time
}

func newBrokerRecord[Payload any](capacity int) *brokerRecord[Payload] {
	mailbox := make(chan Payload, capacity)
	return &brokerRecord[Payload]{mailbox: mailbox, id: uuid.NewV7(), lastUpdated: time.Now()}
}

func (s *brokerRecord[Payload]) touch() {
	s.lastUpdated = time.Now()
}

func (s *brokerRecord[Payload]) subscriber() Subscriber[Payload] {
	return Subscriber[Payload]{id: s.id, mailbox: s.mailbox}
}

type Subscriber[Payload any] struct {
	id      uuid.UUID
	mailbox chan Payload
}

func (s Subscriber[Payload]) Mailbox() chan Payload {
	return s.mailbox
}
