package subscribers

import (
	"math/bits"
	"runtime"
	"time"
)

type BrokerConfig struct {
	BrokerWorkerCount      int
	GlobalBrokerShardCount int
	IdleSubscriberTimeout  time.Duration
	outputChan             chan struct{}
}

var DefaultBrokerConfig = BrokerConfig{
	BrokerWorkerCount:      powerOfTwo(runtime.GOMAXPROCS(0)), // worker and shard count scale with available parallelism
	GlobalBrokerShardCount: powerOfTwo(runtime.GOMAXPROCS(0)),
	IdleSubscriberTimeout:  1 * time.Minute,
}

func powerOfTwo(n int) int {
	return 1 << bits.Len(uint(n-1))
}

var MinimalBrokerConfig = BrokerConfig{
	BrokerWorkerCount:      1,
	GlobalBrokerShardCount: 1,
	IdleSubscriberTimeout:  -1,
}
