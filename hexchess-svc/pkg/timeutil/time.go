package timeutil

import (
	"time"
)

func Every(duration time.Duration, work func()) chan bool {
	ticker := time.NewTicker(duration)
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

	return stop
}
