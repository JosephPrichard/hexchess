package queue

import "fmt"

type NonRetryableQueueError struct {
	Err error
}

func (err NonRetryableQueueError) Error() string {
	return fmt.Sprintf("non-retryable outbox error: %v", err.Err)
}
