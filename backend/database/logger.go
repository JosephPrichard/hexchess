package database

import (
	"cmp"
	"context"
	"log/slog"
	"runtime"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/tracelog"
)

// PgxLogger is an adapter from [slog.Handler] to [tracelog.PgxLogger].
type PgxLogger struct {
	handler slog.Handler
}

// NewTracer return a new [pgx.QueryTracer] instance backed by a [PgxLogger].
func NewTracer(l *slog.Logger) pgx.QueryTracer {
	return &tracelog.TraceLog{
		Logger:   NewPgxLogger(l.Handler()),
		LogLevel: tracelog.LogLevelTrace,
	}
}

// NewPgxLogger return a new [PgxLogger] instance.
func NewPgxLogger(handler slog.Handler) *PgxLogger {
	return &PgxLogger{
		handler: handler,
	}
}

// Log implements the [tracelog.PgxLogger] interface.
func (l *PgxLogger) Log(
	ctx context.Context,
	traceLogLevel tracelog.LogLevel,
	msg string,
	data map[string]any,
) {
	level := translateTraceLogLevel(traceLogLevel)
	if !l.handler.Enabled(ctx, level) {
		return
	}

	attrs := make([]slog.Attr, 0, len(data))
	for k, v := range data {
		attrs = append(attrs, slog.Any(k, v))
	}
	slices.SortFunc(attrs, func(a, b slog.Attr) int {
		return cmp.Compare(a.Key, b.Key)
	})

	// Skip 4 stack frames to log pgx.QueryTracer caller:
	//   - runtime.Callers
	//   - this function
	//   - this function’s caller (TraceLog.log helper)
	//   - QueryTracer hook
	var pcs [1]uintptr
	runtime.Callers(4, pcs[:])

	r := slog.NewRecord(l.now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.handler.Handle(ctx, r)
}

func (l *PgxLogger) now() time.Time {
	return time.Now()
}

func translateTraceLogLevel(level tracelog.LogLevel) slog.Level {
	switch level {
	case tracelog.LogLevelTrace:
		return slog.LevelDebug - 1
	case tracelog.LogLevelDebug:
		return slog.LevelDebug
	case tracelog.LogLevelInfo:
		return slog.LevelInfo
	case tracelog.LogLevelWarn:
		return slog.LevelWarn
	case tracelog.LogLevelError:
		return slog.LevelError
	}
	return 0
}
