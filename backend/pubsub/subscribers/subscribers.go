package subscribers

import (
	"time"
	"uuid"
)

type subscriberRecord[Payload any] struct {
	id                  uuid.UUID
	mailbox             chan Payload
	lastUpdated         time.Time
	slowConsumerTimeout time.Duration
}

func newSubscriberRecord[Payload any](capacity int, slowConsumerTimeout time.Duration) subscriberRecord[Payload] {
	mailbox := make(chan Payload, capacity)
	return subscriberRecord[Payload]{mailbox: mailbox, id: uuid.NewV7(), lastUpdated: time.Now(), slowConsumerTimeout: slowConsumerTimeout}
}

func (s *subscriberRecord[Payload]) touch() {
	s.lastUpdated = time.Now()
}

func (s *subscriberRecord[Payload]) isSlow() bool {
	return time.Now().Sub(s.lastUpdated) > s.slowConsumerTimeout
}

func (s *subscriberRecord[Payload]) toPublic() Subscriber[Payload] {
	return Subscriber[Payload]{id: s.id, mailbox: s.mailbox}
}

type Subscriber[Payload any] struct {
	id      uuid.UUID
	mailbox chan Payload
}

func (s Subscriber[Payload]) Mailbox() chan Payload {
	return s.mailbox
}
