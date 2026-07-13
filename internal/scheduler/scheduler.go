// Package scheduler runs periodic work in-process, guarded by a DB leader lock
// so only one instance acts when several share a Postgres database:
//   - generate due recurring expenses (exactly once),
//   - purge expired sessions/tokens and stale cached currency rates.
// Gated by the SCHEDULER config flag.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/service"
)

// Scheduler ticks periodically and performs maintenance while it holds the
// leader lock.
type Scheduler struct {
	Svc      *service.Service
	Holder   string
	Interval time.Duration
}

// New creates a Scheduler that ticks every minute.
func New(svc *service.Service, holder string) *Scheduler {
	return &Scheduler{Svc: svc, Holder: holder, Interval: time.Minute}
}

// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	slog.Info("scheduler: starting", "holder", s.Holder, "interval", s.Interval)
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler: stopped")
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	st := s.Svc.Store
	leader, err := st.AcquireLock(ctx, "cleanup", s.Holder, 5*s.Interval)
	if err != nil {
		slog.Warn("scheduler: lock error", "err", err)
		return
	}
	if !leader {
		return
	}

	if n, err := s.Svc.GenerateDueRecurrences(ctx); err != nil {
		slog.Warn("scheduler: recurrence generation", "err", err)
	} else if n > 0 {
		slog.Info("scheduler: generated recurring expenses", "count", n)
	}

	if err := st.DeleteExpiredSessions(ctx); err != nil {
		slog.Warn("scheduler: session cleanup", "err", err)
	}
	if err := st.DeleteExpiredTokens(ctx); err != nil {
		slog.Warn("scheduler: token cleanup", "err", err)
	}
	cutoff := time.Now().UTC().Add(-s.Svc.Config.CacheRetentionInterval).Format("2006-01-02")
	if err := st.DeleteRatesBefore(ctx, cutoff); err != nil {
		slog.Warn("scheduler: rate cache cleanup", "err", err)
	}
}
