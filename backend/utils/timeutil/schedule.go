package timeutil

import (
	"math"
	"math/rand/v2"
	"time"
)

func Schedule(duration time.Duration, work func()) func() {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-ticker.C:
				work()
			case <-stop:
				return
			}
		}
	}()

	return func() { stop <- true }
}

func BackoffSleep(retry int, multiplier float64, base time.Duration) {
	backoff := float64(base) * math.Pow(multiplier, float64(retry))
	jitter := rand.Float64() * float64(base)
	time.Sleep(time.Duration(backoff + jitter))
}
