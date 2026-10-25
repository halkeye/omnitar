package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// With stores l in ctx for FromContext.
func With(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the request logger from ctx, or Default with unknown_context set.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return Default.With("unknown_context", true)
}
