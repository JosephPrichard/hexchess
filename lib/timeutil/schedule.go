package timeutil

import "time"

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