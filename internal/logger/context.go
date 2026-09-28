package logger

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
)

type Logger = *logrus.Entry
type Fields = logrus.Fields

var DefaultLogger *logrus.Logger

func init() {
	DefaultLogger = logrus.New()
	DefaultLogger.SetFormatter(&logrus.JSONFormatter{})
	DefaultLogger.SetOutput(os.Stdout)
}

// Define the context key type.
type contextKey string

var loggerKey contextKey = "logger"

func WithValue(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func FromContext(ctx context.Context) Logger {
	val, ok := ctx.Value(loggerKey).(Logger)
	if !ok {
		val = logrus.NewEntry(DefaultLogger).WithField("unknown_context", "unknown_context")
	}
	return val.WithContext(ctx)
}
