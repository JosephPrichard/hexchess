package network

import (
	"context"
	"fmt"
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
	OnClose    func()
	Stream     chan Message
	MapMessage func(Message) SseEvent
}

func streamServerSentEvents[Message any](client *SSEClient, cfg ListenConfig[Message]) {
	go func() {
		<-client.ctx.Done()
		if cfg.OnClose == nil {
			close(cfg.Stream)
		} else {
			cfg.OnClose()
		}
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case message, ok := <-cfg.Stream:
			if !ok {
				return
			}
			if cfg.MapMessage == nil {
				slog.WarnContext(client.ctx, "while streaming messages: mapMessage fn is not provided, dropping message")
				continue
			}
			event := cfg.MapMessage(message)
			if event == (SseEvent{}) {
				continue
			}
			client.SendEvent(event)
		case <-keepAliveTicker.C:
			client.SendEvent(SseEvent{EventName: MetaEvent, EventData: "KeepAlive"})
		}
	}
}

func blockSSEClient(client *SSEClient) {
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
		slog.ErrorContext(client.ctx, "failed to write to sse", "error", err)
	}

	client.f.Flush()

	if event.EventName != "KeepAlive" {
		slog.InfoContext(client.ctx, "writing server sent event", "event", event.EventName, "data", event.EventData)
	}
}
