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

func WithContext(ctx context.Context) Performance {
	var pc [1]uintptr
	n := runtime.Callers(2, pc[:])
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	return Performance{ctx: ctx, name: frame.Function, start: time.Now()}
}

func New() Performance {
	return Performance{start: time.Now()}
}

func (p Performance) Duration(duration *time.Duration) {
	*duration = time.Since(p.start)
}

func (p Performance) Log() {
	slog.InfoContext(p.ctx, "performance measurement", "function", p.name, "timeTaken", time.Since(p.start).String())
}
