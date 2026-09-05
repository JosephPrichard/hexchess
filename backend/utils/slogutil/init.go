package slogutil

import (
	"context"
	"hexchess-svc/utils/config"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func init() {
	stderrHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(stderrHandler))
}

func InitLoggers(serviceName string, oltpEndpoint string, activeProfile config.Profile) func() {
	stderrHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handlers := []slog.Handler{stderrHandler}
	shutdown := func() {}

	if oltpEndpoint != "" {
		otelResource, err := resource.New(context.Background(),
			resource.WithAttributes(semconv.ServiceName(serviceName), semconv.DeploymentEnvironment(activeProfile.String())),
			resource.WithHost(),
			resource.WithProcess(),
		)
		if err != nil {
			Fatal("failed to create OTel resource", err)
		}

		exporter, err := otlploghttp.New(context.Background(),
			otlploghttp.WithEndpoint(oltpEndpoint),
			otlploghttp.WithURLPath("/otlp/v1/logs"),
			otlploghttp.WithInsecure(),
		)
		if err != nil {
			Fatal("failed to create OTel exporter", err)
		}
		provider := sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
			sdklog.WithResource(otelResource),
		)
		global.SetLoggerProvider(provider)

		otelHandler := otelslog.NewLogger("hexchess", otelslog.WithLoggerProvider(provider)).Handler()
		handlers = append(handlers, otelHandler)

		shutdown = func() {
			if err := provider.Shutdown(context.Background()); err != nil {
				slog.Error("OTel provider shutdown error", "error", err)
			}
		}
	}

	slog.SetDefault(slog.New(
		NewLogRecordHandler(serviceName, NewLogFanoutHandler(handlers)),
	))

	slog.Info("finished initializing loggers", "oltpEndpoint", oltpEndpoint)
	return shutdown
}
