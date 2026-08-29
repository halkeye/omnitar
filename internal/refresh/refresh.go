// Package refresh drives periodic re-fetching of the directory from its
// Source into the Store.
package refresh

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/halkeye/omnitar/internal/directory"
)

type Refresher struct {
	Source   directory.Source
	Interval time.Duration
	Logger   *logrus.Logger
}

// Once performs a single fetch-and-swap.
func (r *Refresher) Once(ctx context.Context) error {
	err := r.Source.Fetch(ctx)
	if err != nil {
		return err
	}
	return nil
}

// RunPeriodic loops on Interval, calling Once and logging failures, until
// ctx is canceled. Callers are expected to have already performed an
// initial Once (typically with retries) before calling RunPeriodic.
func (r *Refresher) RunPeriodic(ctx context.Context) error {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.Once(ctx); err != nil {
				r.Logger.WithError(err).WithField("source", r.Source.Name()).
					Error("directory refresh failed; keeping previous snapshot")
				continue
			}
			r.Logger.WithField("people", r.Source.Len()).Info("directory refreshed")
		}
	}
}
