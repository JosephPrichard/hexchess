package redisub

import (
	"time"
	"uuid"
)

type SubscriberID = uuid.UUID

type Subscriber struct {
	id          SubscriberID
	mailbox     chan []byte
	lastUpdated time.Time
}

func newSubscriber(capacity int) *Subscriber {
	mailbox := make(chan []byte, capacity)
	return &Subscriber{mailbox: mailbox, id: uuid.NewV7(), lastUpdated: time.Now()}
}
