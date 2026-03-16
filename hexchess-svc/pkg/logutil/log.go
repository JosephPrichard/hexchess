package logutil

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type TestLogger interface {
	Context() context.Context
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

type TraceType string

var Trace TraceType = "trace"

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

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
	//log.Fatalf("%s: %v", msg, args)
}

func FatalErr(msg string, err error) {
	slog.Error("failed to "+msg, "err", err)
	os.Exit(1)
	//log.Fatalf("%s: %v", msg, err)
}

type TraceHandler struct {
	slog.Handler
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if v := ctx.Value(Trace); v != nil {
		r.Add("trace", v)
	}
	return h.Handler.Handle(ctx, r)
}

var LogWriter io.Writer = os.Stderr

func InitLoggers(f *os.File) {
	if f != nil {
		LogWriter = io.MultiWriter(os.Stderr, f)
	}
	handler := slog.NewTextHandler(LogWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(&TraceHandler{handler}))
}
