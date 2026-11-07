package network

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type SSEClient struct {
	ctx context.Context
	w   http.ResponseWriter
	f   http.Flusher
}

type ListenConfig[Message any] struct {
	OnClose    func()
	MapMessage func(Message) SseEvent
	Stream     chan Message
}

func (client *SSEClient) StreamMessages[Message any](cfg ListenConfig[Message]) {
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

func (client *SSEClient) Block() {
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

func (client *SSEClient) SendEvent(event SseEvent) {
	if _, err := fmt.Fprintf(client.w, "event: %s\ndata: %s\n\n", event.EventName, event.EventData); err != nil {
		slog.ErrorContext(client.ctx, "failed to write to sse", "error", err)
	}

	client.f.Flush()

	if event.EventName != "KeepAlive" {
		slog.InfoContext(client.ctx, "writing server sent event", "event", event.EventName, "data", event.EventData)
	}
}
