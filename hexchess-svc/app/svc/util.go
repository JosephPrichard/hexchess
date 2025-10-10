package svc

import "log/slog"

type TraceType string

var TraceKey TraceType = "trace"

func dynLevel(err error) slog.Level {
	if err != nil {
		return slog.LevelError
	}
	return slog.LevelInfo
}

func dynLog(msg string, err error, args ...any) {
	if err != nil {
		ea := make([]any, len(args)+2)
		ea[0] = "err"
		ea[1] = err
		for i, arg := range args {
			ea[i+2] = arg
		}
		slog.Error(msg, ea...)
	} else {
		slog.Info(msg, args...)
	}
}

func pointerOf[T any](v T) *T {
	return &v
}
