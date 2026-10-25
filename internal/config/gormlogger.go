package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/halkeye/omnitar/internal/logger"

	"gorm.io/gorm"
	gormiologger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type gormslog struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
}

var _ gormiologger.Interface = gormslog{}

func (g gormslog) Error(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf(s, args...), "component", "gormslog")
}

func (g gormslog) Info(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(s, args...), "component", "gormslog")
}

func (g gormslog) LogMode(gormiologger.LogLevel) gormiologger.Interface {
	return g
}

func (g gormslog) Warn(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(s, args...), "component", "gormslog")
}

func (g gormslog) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()
	attrs := []any{"component", "gorm"}
	if g.SourceField != "" {
		attrs = append(attrs, g.SourceField, utils.FileWithLineNum())
	}
	msg := fmt.Sprintf("%s [%s]", sql, elapsed)

	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && g.SkipErrRecordNotFound) {
		logger.FromContext(ctx).ErrorContext(ctx, msg, append(attrs, "error", err)...)
		return
	}

	if g.SlowThreshold != 0 && elapsed > g.SlowThreshold {
		logger.FromContext(ctx).WarnContext(ctx, msg, attrs...)
		return
	}

	logger.FromContext(ctx).Log(ctx, slog.LevelDebug, msg, attrs...)
}
