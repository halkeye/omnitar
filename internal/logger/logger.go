package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var Level slog.LevelVar

// Default is the process-wide JSON logger on stderr.
var Default *slog.Logger

func init() {
	Default = NewJSON(os.Stderr)
}

func jsonHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: &Level})
}

// NewJSON returns a JSON slog.Logger writing to w, using the package Level var.
func NewJSON(w io.Writer) *slog.Logger {
	return slog.New(jsonHandler(w))
}

// ParseLevel parses LOG_LEVEL environment values.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown level %q", s)
	}
}

// SetLevel updates the minimum level for Default and other loggers using Level.
func SetLevel(level slog.Level) {
	Level.Set(level)
}

// Writer adapts l for gin.CustomRecoveryWithWriter and similar APIs.
func Writer(l *slog.Logger) io.Writer {
	if l == nil {
		l = Default
	}
	return &writerAdapter{log: l}
}

type writerAdapter struct {
	log *slog.Logger
}

func (w *writerAdapter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.log.Error(msg)
	}
	return len(p), nil
}
