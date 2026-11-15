package subscribers

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
