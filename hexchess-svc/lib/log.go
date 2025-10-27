package lib

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type TraceType string

var TK TraceType = "trace"

func DynLog(ctx context.Context, msg string, err error, args ...any) {
	if err != nil {
		ea := make([]any, len(args)+2)
		for i, arg := range args {
			ea[i] = arg
		}
		ea[len(args)] = "err"
		ea[len(args)+1] = err
		slog.ErrorContext(ctx, msg, ea...)
	} else {
		slog.InfoContext(ctx, msg, args...)
	}
}

type TraceHandler struct {
	slog.Handler
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if v := ctx.Value(TK); v != nil {
		r.Add("trace", v)
	}
	return h.Handler.Handle(ctx, r)
}

var LogFile io.Writer = os.Stderr

func InitLoggers(f *os.File) {
	if f != nil {
		LogFile = io.MultiWriter(os.Stderr, f)
	}
	handler := slog.NewTextHandler(LogFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(&TraceHandler{handler}))
}
