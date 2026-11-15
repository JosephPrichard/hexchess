package subscribers

import "testing"

func testSubscriber[Event any](sub chan Event, mChan chan []Event) {
	var messages []Event
	for msg := range sub {
		messages = append(messages, msg)
	}
	mChan <- messages
}

func collectMessages(messages [][]byte) []string {
	var strMessages []string
	for _, msg := range messages {
		strMessages = append(strMessages, string(msg))
	}
	return strMessages
}

func recv(t *testing.T, c chan struct{}, n int) {
	for range n {
		select {
		case <-t.Context().Done():
			t.Fatalf("timed out while waiting for channel: %v", t.Context().Err())
		case <-c:
		}
	}
}
