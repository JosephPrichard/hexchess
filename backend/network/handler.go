package network

import (
	"encoding/json/v2"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net/http"
	"time"

	"uuid"
)

func RouteMiddleware(allowedOrigins string) func(handlerFunc http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace := r.Header.Get("X-trace")
			if trace == "" {
				trace = uuid.NewV7().String()
			}
			r = r.WithContext(slogutil.TracedContext(r.Context(), trace))

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-trace, Content-Digest, Rollout")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Rest(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		start := time.Now()
		slog.InfoContext(ctx, "received REST call", "method", r.Method, "path", r.URL.Path, "url", r.URL.String(), "headers", r.Header)

		if err := h(w, r); err != nil {
			resp := ServiceViewFromErr(err)
			writeJSON(ctx, w, resp.Status, resp)

			slog.Log(ctx, LevelFromStatus(resp.Status), "failed to handle REST call", "error", err, "path", r.URL.Path, "status", resp.Status, "timeTaken", time.Since(start).String())
		} else {
			slog.InfoContext(ctx, "completed REST call", "path", r.URL.Path, "timeTaken", time.Since(start).String())
		}
	}
}

func Json(object any) http.HandlerFunc {
	bytes, err := json.Marshal(object)
	if err != nil {
		slogutil.Fatal("marshal static object", err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(bytes); err != nil {
			slog.ErrorContext(r.Context(), "write json", "error", err)
		}
	}
}

func SSE(h func(w *SSEClient, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "received sse request", "method", r.Method, "url", r.URL)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is unsupported", http.StatusInternalServerError)
			return
		}
		ctx := r.Context()

		sseClient := NewSSEClient(ctx, w, f)

		if err := h(sseClient, r); err != nil {
			resp := ServiceViewFromErr(err)

			slog.Log(ctx, LevelFromStatus(resp.Status), "sse request failed", "error", err, "method", r.Method, "url", r.URL)

			sseClient.SendEvent(SseEvent{EventName: MetaEvent, EventData: resp.Message})
		}
		slog.InfoContext(ctx, "finished sse request", "method", r.Method, "url", r.URL)
	}
}
