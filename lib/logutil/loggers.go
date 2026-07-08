package logutil

import (
	"context"
	"errors"
	"log/slog"
)

type LogRecordHandler struct {
	slog.Handler
}

var PropagatedLogKeys = []string{Trace, SessionID, MessageID, GroupID, EventID, RequestID}

func (h *LogRecordHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, key := range PropagatedLogKeys {
		if v := ctx.Value(key); v != nil {
			r.Add(key, v)
		}
	}
	return h.Handler.Handle(ctx, r)
}

type LogFanoutHandler struct {
	handlers []slog.Handler
}

func (f *LogFanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *LogFanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *LogFanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &LogFanoutHandler{handlers}
}

func (f *LogFanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &LogFanoutHandler{handlers}
}
