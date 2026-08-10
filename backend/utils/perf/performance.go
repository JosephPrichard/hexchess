package perf

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

type Performance struct {
	ctx   context.Context
	name  string
	start time.Time
}

func getCaller() string {
	var pc [1]uintptr
	n := runtime.Callers(3, pc[:])
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function
}

func WithContext(ctx context.Context) Performance {
	return Performance{ctx: ctx, name: getCaller(), start: time.Now()}
}

func New() Performance {
	return Performance{ctx: context.Background(), name: getCaller(), start: time.Now()}
}

func (p Performance) Duration(duration *time.Duration) {
	*duration = time.Since(p.start)
}

func (p Performance) Log() {
	slog.InfoContext(p.ctx, "performance measurement", "function", p.name, "timeTaken", time.Since(p.start).String())
}
