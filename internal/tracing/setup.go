package tracing

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/halkeye/omnitar/internal/logger"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// SetupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Create trace exporter using environment variables
	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}

	info, _ := debug.ReadBuildInfo()
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),      // Discover and provide attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME environment variables.
		resource.WithTelemetrySDK(), // Discover and provide information about the OpenTelemetry SDK used.
		resource.WithProcess(),      // Discover and provide process information.
		resource.WithOS(),           // Discover and provide OS information.
		resource.WithContainer(),    // Discover and provide container information.
		resource.WithHost(),         // Discover and provide host information.
		resource.WithAttributes(
			attribute.String("service.name", "omnitar"),
			attribute.String("service.version", info.Main.Version),
		), // Add custom resource attributes.
	)
	if errors.Is(err, resource.ErrPartialResource) || errors.Is(err, resource.ErrSchemaURLConflict) {
		slog.WarnContext(ctx, "error managing resources", "error", err)
	} else if err != nil {
		handleErr(err)
		return shutdown, err
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(spanExporter),
		trace.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	// Create meter provider with the reader
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metricReader),
		metric.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Logs export only when OTEL_LOGS_EXPORTER is set; otherwise autoexport defaults to OTLP localhost.
	if logsExporterConfigured() {
		loggerProvider, err := newLoggerProvider(ctx, res)
		if err != nil {
			handleErr(err)
			return shutdown, err
		}
		shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
		otel.SetLoggerProvider(loggerProvider)

		logger.UseOpenTelemetry("github.com/halkeye/omnitar",
			otelslog.WithVersion(info.Main.Version),
			otelslog.WithSchemaURL(semconv.SchemaURL),
		)
	}

	return shutdown, err
}

func logsExporterConfigured() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_LOGS_EXPORTER"))) {
	case "", "none":
		return false
	default:
		return true
	}
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*log.LoggerProvider, error) {
	logExporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, err
	}

	return log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	), nil
}
