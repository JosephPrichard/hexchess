package subscribers

import (
	"time"
)

type BrokerConfig struct {
	IdleSubscriberTimeout time.Duration
}

var DefaultBrokerConfig = BrokerConfig{
	IdleSubscriberTimeout: 1 * time.Minute,
}

var MinimalBrokerConfig = BrokerConfig{
	IdleSubscriberTimeout: -1,
}
