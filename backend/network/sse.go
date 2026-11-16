package network

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/pubsub/subscribers"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type SSEClient struct {
	lock sync.Mutex

	ctx context.Context
	w   http.ResponseWriter
	f   http.Flusher
}

func NewSSEClient(ctx context.Context, w http.ResponseWriter, f http.Flusher) *SSEClient {
	return &SSEClient{ctx: ctx, w: w, f: f}
}

type ListenConfig[Message any] struct {
	Mailbox    chan Message
	MapMessage func(Message) SseEvent
}

func channelToSSE[Message any](client *SSEClient, mailbox chan Message, mapMessage func(Message) SseEvent) {
	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-client.ctx.Done():
			slog.InfoContext(client.ctx, "SSE: close from client")
			return
		case message, ok := <-mailbox:
			if !ok {
				slog.InfoContext(client.ctx, "SSE: close from server")
				return
			}

			if mapMessage == nil {
				slog.WarnContext(client.ctx, "SSE: while streaming messages: mapMessage fn is not provided, dropping message")
				continue
			}
			event := mapMessage(message)
			if event == (SseEvent{}) {
				continue
			}

			client.SendEvent(event)
		case <-keepAliveTicker.C:
			client.SendEvent(SseEvent{EventName: MetaEvent, EventData: "KeepAlive"})
		}
	}
}

func blockSSE(client *SSEClient) {
	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case <-client.ctx.Done():
			return
		case <-keepAliveTicker.C:
			client.SendEvent(SseEvent{EventName: MetaEvent, EventData: "KeepAlive"})
		}
	}
}

type SseEvent struct {
	EventName string `json:"eventName"`
	EventData string `json:"eventData"`
}

// SendEvent is a thread-safe wrapper to write an event to the client and flush it immediately
// generally SendEvent is intended to be called sync, but locking is good practice to prevent mistakes
func (client *SSEClient) SendEvent(event SseEvent) {
	client.lock.Lock()
	defer client.lock.Unlock()

	if _, err := fmt.Fprintf(client.w, "event: %s\ndata: %s\n\n", event.EventName, event.EventData); err != nil {
		slog.ErrorContext(client.ctx, "SSE: failed to write to sse", "error", err)
	}

	client.f.Flush()

	if event.EventName != "KeepAlive" {
		slog.InfoContext(client.ctx, "SSE: writing server sent event", "event", event.EventName, "data", event.EventData)
	}
}

func sendCountEvent(client *SSEClient, kind subscribers.CountEventKind, count int64) {
	bytes, err := json.Marshal(publishers.CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(client.ctx, "marshal count event", "Err", err)
	}
	client.SendEvent(mapGlobalEvent(subscribers.GlobalCastEvent{Kind: kind, Data: string(bytes)}))
}
