package slogutil

import (
	"context"
	"log/slog"
	"os"
)

type TestLogger interface {
	Context() context.Context
	Logf(format string, args ...any)
	Fatalf(format string, args ...any)
}

type (
	TraceType     string
	MessageIDType string
	SessionIDType string
)

var (
	Trace     = "trace"
	MessageID = "messageID"

	ContextMap = "ctxmap"
)

func SetCtxMapKey(ctx context.Context, key string, value string) context.Context {
	ctxmap, _ := ctx.Value(ContextMap).(map[string]string)

	if ctxmap == nil {
		ctxmap = make(map[string]string)
	}
	ctxmap[key] = value

	return context.WithValue(ctx, ContextMap, ctxmap)
}

func TracedContext(ctx context.Context, trace string) context.Context {
	return SetCtxMapKey(ctx, Trace, trace)
}

func Fatal(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}
	slog.Error("failed to "+msg, args...)
	os.Exit(1)
}

func ErrorLevel(err error) slog.Level {
	if err != nil {
		return slog.LevelError
	} else {
		return slog.LevelInfo
	}
}

func ErrorsLevel(errs []error) slog.Level {
	for _, err := range errs {
		if err != nil {
			return slog.LevelError
		}
	}
	return slog.LevelInfo
}
