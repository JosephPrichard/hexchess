package web

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/pubsub"
	"log/slog"
	"net/http"
)

type SSEClient struct {
	ctx context.Context
	w   http.ResponseWriter
	f   http.Flusher
}

func (sse *SSEClient) event(eventName string, eventData string) {
	if _, err := fmt.Fprintf(sse.w, "event: %s\ndata: %s\n\n", eventName, eventData); err != nil {
		slog.ErrorContext(sse.ctx, "failed to write to sse", "error", err)
	}

	sse.f.Flush()

	slog.InfoContext(sse.ctx, "writing server sent event", "event", eventName, "data", eventData)
}

func writeGlobalEvent(client *SSEClient, event pubsub.GlobalCastEvent) {
	var e string
	switch event.Kind {
	case pubsub.GlobalActiveEvent:
		e = ActiveCountEvent
	case pubsub.GlobalGamesEvent:
		e = GamesCountEvent
	default:
		slog.ErrorContext(client.ctx, "unknown count event key", "event", e)
		return
	}
	client.event(e, event.Data)
}

func writeCountEvent(client *SSEClient, kind pubsub.GlobalEventKind, count int64) {
	b, err := json.Marshal(pubsub.CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(client.ctx, "marshal count event", "Err", err)
	}
	writeGlobalEvent(client, pubsub.GlobalCastEvent{Kind: kind, Data: string(b)})
}
