package subscribers

import (
	"time"
	"uuid"
)

type SubscriberID = uuid.UUID

type subscriberRecord[Payload any] struct {
	id          SubscriberID
	mailbox     chan Payload
	lastUpdated time.Time
}

func newBrokerRecord[Payload any](capacity int) *subscriberRecord[Payload] {
	mailbox := make(chan Payload, capacity)
	return &subscriberRecord[Payload]{mailbox: mailbox, id: uuid.NewV7(), lastUpdated: time.Now()}
}

func (s *subscriberRecord[Payload]) touch() {
	s.lastUpdated = time.Now()
}

func (s *subscriberRecord[Payload]) subscriber() Subscriber[Payload] {
	return Subscriber[Payload]{id: s.id, mailbox: s.mailbox}
}

type Subscriber[Payload any] struct {
	id      SubscriberID
	mailbox chan Payload
}

func (s Subscriber[Payload]) Mailbox() chan Payload {
	return s.mailbox
}
