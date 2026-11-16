package subscribers

import (
	"context"
	"testing"
	"time"
)

func testSubscriber[Event any](sub chan Event, mChan chan []Event) {
	var messages []Event
	for msg := range sub {
		messages = append(messages, msg)
	}
	mChan <- messages
}

func getMessages(t *testing.T, messagesChan chan [][]byte) []string {
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	var strMessages []string

	select {
	case <-ctx.Done():
		t.Fatalf("timed out while waiting for messages: %v", ctx.Err())
	case messages := <-messagesChan:
		for _, msg := range messages {
			strMessages = append(strMessages, string(msg))
		}
	}

	return strMessages
}

func get(t *testing.T, c chan struct{}, n int) {
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	for range n {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out while waiting for channel: %v", ctx.Err())
		case <-c:
		}
	}
}
