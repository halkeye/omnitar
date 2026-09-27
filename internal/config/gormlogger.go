package config

import (
	"context"
	"errors"
	"time"

	"github.com/halkeye/omnitar/internal/logger"

	"gorm.io/gorm"
	gormiologger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type gormlogrus struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
}

var _ gormiologger.Interface = gormlogrus{}

// Error implements [logger.Interface].
func (g gormlogrus) Error(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).WithContext(ctx).WithField("component", "gormlogrus").Errorf(s, args)
}

// Info implements [logger.Interface].
func (g gormlogrus) Info(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).WithContext(ctx).WithField("component", "gormlogrus").Infof(s, args)
}

// LogMode implements [logger.Interface].
func (g gormlogrus) LogMode(gormiologger.LogLevel) gormiologger.Interface {
	// noop for now
	return g
}

// Warn implements [logger.Interface].
func (g gormlogrus) Warn(ctx context.Context, s string, args ...interface{}) {
	logger.FromContext(ctx).WithContext(ctx).WithField("component", "gormlogrus").Warn(s, args)
}

// Trace implements [logger.Interface].
func (g gormlogrus) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()
	fields := logger.Fields{"component": "gorm"}
	if g.SourceField != "" {
		fields[g.SourceField] = utils.FileWithLineNum()
	}
	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && g.SkipErrRecordNotFound) {
		logger.FromContext(ctx).WithError(err).WithFields(fields).Errorf("%s [%s]", sql, elapsed)
		return
	}

	if g.SlowThreshold != 0 && elapsed > g.SlowThreshold {
		logger.FromContext(ctx).WithFields(fields).Warnf("%s [%s]", sql, elapsed)
		return
	}

	logger.FromContext(ctx).WithFields(fields).Debugf("%s [%s]", sql, elapsed)
}
