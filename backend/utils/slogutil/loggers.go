package slogutil

import (
	"context"
	"errors"
	"hexchess-svc/utils/errutil"
	"log/slog"
	"os"
)

type LogRecordHandler struct {
	slog.Handler
	staticLogData
}

type staticLogData struct {
	serviceName     string
	awsRegion       string
	awsExecutionEnv string
	*ecsTaskMetadataBody
}

func NewLogRecordHandler(serviceName string, h slog.Handler) slog.Handler {
	return &LogRecordHandler{
		Handler:             h,
		serviceName:         serviceName,
		awsRegion:           os.Getenv("AWS_REGION"),
		awsExecutionEnv:     os.Getenv("AWS_EXECUTION_ENV"),
		ecsTaskMetadataBody: getEcsMetadata(),
	}
}

func (h *LogRecordHandler) Handle(ctx context.Context, r slog.Record) error {
	ctxmap, ok := ctx.Value(ContextMap).(map[string]string)
	if ok {
		for key, value := range ctxmap {
			r.Add(key, value)
		}
	}

	// propagates common AWS environment data into the logs for easier debugging. if details are not provided, does not fail
	if h.staticLogData.serviceName != "" {
		r.Add("serviceName", h.staticLogData.serviceName)
	}
	if h.staticLogData.awsRegion != "" {
		r.Add("awsRegion", h.staticLogData.awsRegion)
	}
	if h.staticLogData.awsExecutionEnv != "" {
		r.Add("awsExecutionEnv", h.staticLogData.awsExecutionEnv)
	}
	if h.staticLogData.ecsTaskMetadataBody != nil {
		r.Add("ecsTaskMetadata", h.staticLogData.ecsTaskMetadataBody)
	}

	// walks each service error tree provided in the log invocation and flattens it into slog key value pairs
	serviceErrorValues := make(map[string]any)

	r.Attrs(func(attr slog.Attr) bool {
		value := attr.Value.Any()
		if serviceError, ok := value.(*errutil.ServiceError); ok {
			serviceError.Walk(serviceErrorValues)
		}
		return true
	})

	for key, value := range serviceErrorValues {
		r.Add(key, value)
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
