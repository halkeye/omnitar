package logger

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// UseOpenTelemetry tees Default to stderr JSON and otelslog (global LoggerProvider).
// Call after otel.SetLoggerProvider.
func UseOpenTelemetry(name string, opts ...otelslog.Option) {
	Default = slog.New(&multiHandler{
		level: &Level,
		handlers: []slog.Handler{
			jsonHandler(os.Stderr),
			otelslog.NewHandler(name, opts...),
		},
	})
	slog.SetDefault(Default)
}
