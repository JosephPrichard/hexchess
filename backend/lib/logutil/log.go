package logutil

import (
	"context"
	"errors"
	"hexchess-svc/lib/serrors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type TestLogger interface {
	Context() context.Context
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...any)
}

type TraceType string

var Trace TraceType = "trace"

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

func RootLog(ctx context.Context, level slog.Level, msg string, err error, args ...any) {
	args = append(args, "error", err)
	serrors.WalkValues(err, &args)
	slog.Log(ctx, level, msg, args...)
}

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func FatalErr(msg string, err error) {
	slog.Error("failed to "+msg, "error", err)
	os.Exit(1)
}

type TraceHandler struct {
	slog.Handler
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if v := ctx.Value(Trace); v != nil {
		r.Add("trace", v)
	}
	return h.Handler.Handle(ctx, r)
}

type FanoutHandler struct {
	handlers []slog.Handler
}

func (f *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
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

func (f *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &FanoutHandler{handlers}
}

func (f *FanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &FanoutHandler{handlers}
}

type LogConfig struct {
	OtlpEndpoint string
}

func InitLoggers(config LogConfig) func(ctx context.Context) {
	stderrHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handlers := []slog.Handler{stderrHandler}
	shutdown := func(ctx context.Context) {}

	if config.OtlpEndpoint != "" {
		slog.Info("starting OTel rpc slog bridge logger", "config", config)

		exporter, err := otlploggrpc.New(context.Background(),
			otlploggrpc.WithEndpoint(config.OtlpEndpoint),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			slog.Error("failed to create OTLP exporter, OTel logging disabled", "error", err)
		}
		provider := sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		)
		global.SetLoggerProvider(provider)

		otelHandler := otelslog.NewLogger("hexchess", otelslog.WithLoggerProvider(provider)).Handler()

		handlers = append(handlers, otelHandler)

		shutdown = func(ctx context.Context) {
			if err := provider.Shutdown(ctx); err != nil {
				slog.Error("OTel provider shutdown error", "error", err)
			}
		}
	}

	slog.SetDefault(slog.New(
		&TraceHandler{&FanoutHandler{handlers: handlers}},
	))

	return shutdown
}
