// Package worker runs background jobs that drive the application on a timer.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// UserCounter returns the total number of users.
type UserCounter interface {
	Count(ctx context.Context) (int64, error)
}

// UserCountReporter periodically logs the total number of users.
type UserCountReporter struct {
	counter  UserCounter
	interval time.Duration
	logger   *slog.Logger
}

// NewUserCountReporter creates a reporter that runs every interval.
func NewUserCountReporter(counter UserCounter, interval time.Duration, logger *slog.Logger) *UserCountReporter {
	return &UserCountReporter{counter: counter, interval: interval, logger: logger}
}

// Run logs the user count every interval until ctx is cancelled.
func (r *UserCountReporter) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.report(ctx)
		}
	}
}

func (r *UserCountReporter) report(ctx context.Context) {
	// A slow query must not overlap with the next tick.
	ctx, cancel := context.WithTimeout(ctx, r.interval)
	defer cancel()

	n, err := r.counter.Count(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return // shutting down
		}
		r.logger.Error("count users failed", "error", err)
		return
	}
	r.logger.Info("user count", "total", n)
}
