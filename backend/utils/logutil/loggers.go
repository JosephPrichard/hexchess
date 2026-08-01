package logutil

import (
	"context"
	"errors"
	"log/slog"
	"os"
)

type LogRecordHandler struct {
	slog.Handler
	staticLogData
}

type staticLogData struct {
	awsRegion       string
	awsExecutionEnv string
	*ecsTaskMetadataBody
}

var PropagatedLogKeys = []string{Trace, SessionID, MessageID, GroupID, EventID, RequestID}

func NewLogRecordHandler(h slog.Handler) slog.Handler {
	return &LogRecordHandler{
		Handler: h,
		staticLogData: staticLogData{
			awsRegion:           os.Getenv("AWS_REGION"),
			awsExecutionEnv:     os.Getenv("AWS_EXECUTION_ENV"),
			ecsTaskMetadataBody: getEcsMetadata(),
		},
	}
}

func (h *LogRecordHandler) Handle(ctx context.Context, r slog.Record) error {
	// propagates only specific log keys inserted into the context.
	// the reason why we do not propagate ALL keys is that it can be expensive to walk a large context tree
	for _, key := range PropagatedLogKeys {
		if v := ctx.Value(key); v != nil {
			r.Add(key, v)
		}
	}

	// propagates common AWS environment data into the logs for easier debugging. if details are not provided, does not fail
	if h.staticLogData.awsRegion != "" {
		r.Add("awsRegion", h.staticLogData.awsRegion)
	}
	if h.staticLogData.awsExecutionEnv != "" {
		r.Add("awsExecutionEnv", h.staticLogData.awsExecutionEnv)
	}
	if h.staticLogData.ecsTaskMetadataBody != nil {
		r.Add("ecsTaskMetadata", h.staticLogData.ecsTaskMetadataBody)
	}

	return h.Handler.Handle(ctx, r)
}

type LogFanoutHandler struct {
	handlers []slog.Handler
}

func NewLogFanoutHandler(handlers []slog.Handler) slog.Handler {
	return &LogFanoutHandler{handlers: handlers}
}

func (f *LogFanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *LogFanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *LogFanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &LogFanoutHandler{handlers}
}

func (f *LogFanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &LogFanoutHandler{handlers}
}
