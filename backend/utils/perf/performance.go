package perf

import (
	"context"
	"errors"
	"fmt"
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
	// offset of 3, one for this function, one for the caller inside "perf", one for the caller outside "perf"
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

// LogFailable is a variant of Log that allows the caller to record of the operation that is being measured failed or not
func (p Performance) LogFailable(errPtr *error) {
	var err error
	if errPtr != nil {
		err = *errPtr
	}

	args := []any{"function", p.name, "timeTaken", time.Since(p.start).String()}

	if err != nil {
		leafErr := leafError(err)
		leafErrType := fmt.Sprintf("%T", leafErr)
		args = append(args, "error", err, "cause", leafErr, "causeType", leafErrType)
	}

	slog.InfoContext(p.ctx, "performance measurement", args...)
}

func leafError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}
