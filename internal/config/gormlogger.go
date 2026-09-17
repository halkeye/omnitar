package config

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type gormlogrus struct {
	logger *logrus.Logger

	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
}

var _ gormlogger.Interface = gormlogrus{}

// Error implements [logger.Interface].
func (g gormlogrus) Error(ctx context.Context, s string, args ...interface{}) {
	g.logger.WithContext(ctx).WithField("component", "logrus").Errorf(s, args)
}

// Info implements [logger.Interface].
func (g gormlogrus) Info(ctx context.Context, s string, args ...interface{}) {
	g.logger.WithContext(ctx).WithField("component", "logrus").Infof(s, args)
}

// LogMode implements [logger.Interface].
func (g gormlogrus) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	// noop for now
	return g
}

// Warn implements [logger.Interface].
func (g gormlogrus) Warn(ctx context.Context, s string, args ...interface{}) {
	g.logger.WithContext(ctx).WithField("component", "logrus").Warn(s, args)
}

// Trace implements [logger.Interface].
func (g gormlogrus) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()
	fields := logrus.Fields{
		"component": "gorm",
	}
	if g.SourceField != "" {
		fields[g.SourceField] = utils.FileWithLineNum()
	}
	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && g.SkipErrRecordNotFound) {
		fields[logrus.ErrorKey] = err
		g.logger.WithContext(ctx).WithFields(fields).Errorf("%s [%s]", sql, elapsed)
		return
	}

	if g.SlowThreshold != 0 && elapsed > g.SlowThreshold {
		g.logger.WithContext(ctx).WithFields(fields).Warnf("%s [%s]", sql, elapsed)
		return
	}

	g.logger.WithContext(ctx).WithFields(fields).Debugf("%s [%s]", sql, elapsed)
}
