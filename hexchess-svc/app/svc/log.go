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
