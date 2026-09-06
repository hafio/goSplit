// Package scheduler runs periodic work in-process, guarded by a DB leader lock
// so only one instance acts when several share a Postgres database:
//   - generate due recurring expenses (exactly once),
//   - purge expired sessions/tokens and stale cached currency rates,
//   - take a scheduled backup on a cron rule, pruning to a retention count.
//
// Gated by the SCHEDULER config flag.
package scheduler

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/service"
)

// backupLockID is deliberately separate from the cleanup lock. The cleanup
// lock's TTL is a small multiple of the tick interval, which a real backup can
// easily outlive -- it would lose leadership mid-write.
const backupLockID = "scheduled-backup"

// backupLockTTL is renewed for as long as the dump runs.
const backupLockTTL = 10 * time.Minute

// Scheduler ticks periodically and performs maintenance while it holds the
// leader lock.
type Scheduler struct {
	Svc      *service.Service
	Holder   string
	Interval time.Duration

	// backupSchedule is nil when BACKUP_CRON is unset. nextBackupAt is held in
	// memory: a window missed across a restart is not a correctness problem,
	// and the leader lock is what prevents a double fire.
	backupSchedule cron.Schedule
	nextBackupAt   time.Time
	backupRunning  atomic.Bool
}

// New creates a Scheduler that ticks every minute.
func New(svc *service.Service, holder string) *Scheduler {
	s := &Scheduler{Svc: svc, Holder: holder, Interval: time.Minute}
	if rule := svc.Config.BackupCron; rule != "" {
		// config.Load already validated this, so a failure here means the
		// config was bypassed. Log rather than panic: a bad cron rule should
		// not take down an instance that is otherwise healthy.
		sched, err := config.BackupSchedule(rule)
		if err != nil {
			slog.Error("scheduler: BACKUP_CRON is invalid, scheduled backups disabled", "rule", rule, "err", err)
		} else {
			s.backupSchedule = sched
			s.nextBackupAt = sched.Next(time.Now().UTC())
			slog.Info("scheduler: scheduled backups enabled",
				"cron", rule, "dir", svc.Config.BackupDir,
				"retention", svc.Config.BackupRetentionCount, "next", s.nextBackupAt)
		}
	}
	return s
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

	s.maybeBackup(ctx)
}

// maybeBackup starts a scheduled backup when one is due.
//
// The dump runs on its own goroutine, not inline: it can take minutes, and
// tick() must stay responsive so recurrence generation and cache cleanup are
// not delayed behind it. nextBackupAt advances before the goroutine starts, so
// a slow dump cannot be started twice by the following tick.
func (s *Scheduler) maybeBackup(ctx context.Context) {
	if s.backupSchedule == nil {
		return
	}
	now := time.Now().UTC()
	if now.Before(s.nextBackupAt) {
		return
	}
	s.nextBackupAt = s.backupSchedule.Next(now)

	if !s.backupRunning.CompareAndSwap(false, true) {
		slog.Warn("scheduler: previous scheduled backup still running, skipping this window",
			"next", s.nextBackupAt)
		return
	}
	go func() {
		defer s.backupRunning.Store(false)
		s.runBackup(ctx)
	}()
}

// runBackup takes the backup under its own renewed lock, then prunes.
func (s *Scheduler) runBackup(ctx context.Context) {
	st := s.Svc.Store
	cfg := s.Svc.Config

	holder := s.Holder
	if host, err := os.Hostname(); err == nil {
		holder = host + "-" + s.Holder
	}

	leader, err := st.AcquireLock(ctx, backupLockID, holder, backupLockTTL)
	if err != nil {
		slog.Warn("scheduler: backup lock error", "err", err)
		return
	}
	if !leader {
		// Another instance is taking this window's backup.
		return
	}

	// Keep the lock fresh for as long as the dump runs, so a slow dump cannot
	// let another replica start a second one.
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(backupLockTTL / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := st.AcquireLock(ctx, backupLockID, holder, backupLockTTL); err != nil {
					slog.Warn("scheduler: backup lock renewal failed", "err", err)
				}
			}
		}
	}()

	start := time.Now()
	r := backup.New(st, cfg)
	path, man, err := r.DumpToDir(ctx, cfg.BackupDir, backup.ArchivePrefix)
	if err != nil {
		slog.Error("scheduler: scheduled backup failed", "err", err)
		return
	}
	slog.Info("scheduler: scheduled backup written",
		"path", path,
		"tables", len(man.Tables),
		"uploads", man.UploadFileCount,
		"took", time.Since(start).Round(time.Millisecond))

	if n, err := backup.PruneOldArchives(cfg.BackupDir, backup.ArchivePrefix, cfg.BackupRetentionCount); err != nil {
		slog.Warn("scheduler: pruning old archives", "removed", n, "err", err)
	} else if n > 0 {
		slog.Info("scheduler: pruned old archives", "removed", n, "kept", cfg.BackupRetentionCount)
	}
	// Pre-restore safety dumps are retained on the same count, so a series of
	// restores cannot fill the volume.
	if _, err := backup.PruneOldArchives(cfg.BackupDir, backup.SafetyDumpPrefix, cfg.BackupRetentionCount); err != nil {
		slog.Warn("scheduler: pruning old safety dumps", "err", err)
	}
}
