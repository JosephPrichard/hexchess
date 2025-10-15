package util

import "log/slog"

type TraceType string

var TraceKey TraceType = "trace"

func DynLog(msg string, err error, args ...any) {
	if err != nil {
		ea := make([]any, len(args)+2)
		for i, arg := range args {
			ea[i] = arg
		}
		ea[len(args)] = "err"
		ea[len(args)+1] = err
		slog.Error(msg, ea...)
	} else {
		slog.Info(msg, args...)
	}
}
