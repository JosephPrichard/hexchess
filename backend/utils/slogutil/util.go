package slogutil

import (
	"context"
	"hexchess-svc/utils/errutil"
	"log/slog"
	"os"
)

type TestLogger interface {
	Context() context.Context
	Logf(format string, args ...interface{})
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
	SessionID = "sessionID"
	GroupID   = "groupID"
	EventID   = "eventID"
	RequestID = "requestID"

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

func Log(ctx context.Context, msg string, err error, args ...any) {
	if err != nil {
		ea := make([]any, len(args)+2)
		copy(ea, args)
		ea[len(args)] = "error"
		ea[len(args)+1] = err
		slog.ErrorContext(ctx, msg, ea...)
	} else {
		slog.InfoContext(ctx, msg, args...)
	}
}

func Error(ctx context.Context, level slog.Level, msg string, err error, args ...any) {
	args = append(args, "error", err)
	errutil.WalkValues(err, &args)
	slog.Log(ctx, level, msg, args...)
}

func Fatal(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}
	slog.Error("failed to "+msg, args...)
	os.Exit(1)
}
