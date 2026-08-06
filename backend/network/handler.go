package network

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func RouteMiddleware(allowedOrigins string) func(handlerFunc http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace := r.Header.Get("X-trace")
			if trace == "" {
				trace = uuid.NewString()
			}
			r = r.WithContext(context.WithValue(r.Context(), logutil.Trace, trace))

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
		slog.InfoContext(ctx, "received REST call",
			"method", r.Method, "path", r.URL.Path, "url", r.URL.String(), "headers", r.Header)

		if err := h(w, r); err != nil {
			resp := ServiceViewFromErr(err)
			writeJSON(w, resp.Status, resp)

			logutil.Error(ctx, LevelFromStatus(resp.Status), "failed to handle REST call", err,
				"path", r.URL.Path, "status", resp.Status, "timeTaken", time.Since(start).String())
		} else {
			slog.InfoContext(ctx, "completed REST call", "path", r.URL.Path, "timeTaken", time.Since(start).String())
		}
	}
}

func Json(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(v)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(b); err != nil {
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
		if err := h(&SSEClient{ctx, w, f}, r); err != nil {
			resp := ServiceViewFromErr(err)

			logutil.Error(ctx, LevelFromStatus(resp.Status), "sse request failed", err, "method", r.Method, "url", r.URL)

			http.Error(w, fmt.Sprintf("%s:%s", MetaEvent, resp.Message), resp.Status)
		}
		slog.InfoContext(ctx, "finished sse request", "method", r.Method, "url", r.URL)
	}
}
