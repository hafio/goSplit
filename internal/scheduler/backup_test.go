package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
)

// newBackupSchedService builds a service whose config has scheduled backups
// configured, so the backup path in tick() is reachable.
func newBackupSchedService(t *testing.T, cron string, retention int) (*service.Service, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		BaseURL:                "http://test.local",
		DatabaseURL:            "file:" + filepath.Join(dir, "sched.db"),
		Engine:                 config.EngineSQLite,
		CacheRetentionInterval: 24 * time.Hour,
		// A dump seals with a key derived from this, so it must be set.
		SessionSecret:        "test-session-secret-0123456789abcdef",
		AppVersion:           "v0.0.0-test",
		UploadDir:            filepath.Join(dir, "uploads"),
		BackupDir:            filepath.Join(dir, "backups"),
		BackupCron:           cron,
		BackupRetentionCount: retention,
		UploadMaxFileSizeMB:  5,
	}
	st, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := service.New(st, &mail.LogMailer{}, cfg)
	svc.SetSynchronousEmail()
	return svc, cfg
}

// archiveCount counts real archives in a directory.
func archiveCount(t *testing.T, dir, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read %s: %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix+"-") && strings.HasSuffix(e.Name(), backup.ArchiveExt) {
			n++
		}
	}
	return n
}

// TestNewWithoutCronDisablesBackups asserts scheduled backups stay off unless
// asked for. They write files, so they must never enable themselves.
func TestNewWithoutCronDisablesBackups(t *testing.T) {
	svc, _ := newBackupSchedService(t, "", 7)
	s := New(svc, "holder")
	if s.backupSchedule != nil {
		t.Error("an empty BACKUP_CRON produced a schedule")
	}
	if !s.nextBackupAt.IsZero() {
		t.Error("nextBackupAt was set with no schedule")
	}
	// tick() must be a no-op for backups, not a nil dereference.
	s.tick(context.Background())
}

// TestNewWithCronSchedulesNextRun asserts a valid rule is parsed and the first
// window is in the future.
func TestNewWithCronSchedulesNextRun(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "0 4 * * *", 7)
	s := New(svc, "holder")
	if s.backupSchedule == nil {
		t.Fatal("BACKUP_CRON was not parsed")
	}
	if !s.nextBackupAt.After(time.Now().UTC().Add(-time.Second)) {
		t.Errorf("nextBackupAt = %v, want a future time", s.nextBackupAt)
	}
	// Nothing has run yet.
	if n := archiveCount(t, cfg.BackupDir, backup.ArchivePrefix); n != 0 {
		t.Errorf("%d archives exist before any window elapsed", n)
	}
}

// TestNewWithInvalidCronDisablesBackupsWithoutPanicking asserts a rule that
// bypassed config validation degrades to "no scheduled backups" rather than
// taking down an otherwise healthy instance.
func TestNewWithInvalidCronDisablesBackupsWithoutPanicking(t *testing.T) {
	svc, _ := newBackupSchedService(t, "not a cron rule", 7)
	s := New(svc, "holder")
	if s.backupSchedule != nil {
		t.Error("an invalid rule produced a schedule")
	}
	s.tick(context.Background())
}

// TestScheduledBackupRunsWhenDue drives the real path: force the window open,
// tick, and wait for the archive. It covers runBackup end to end, including the
// lock acquisition and the dump.
func TestScheduledBackupRunsWhenDue(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "* * * * *", 7)
	s := New(svc, "leader")

	// Make the window already due rather than waiting a minute for it.
	s.nextBackupAt = time.Now().UTC().Add(-time.Second)
	s.tick(context.Background())

	waitFor(t, 10*time.Second, func() bool {
		return archiveCount(t, cfg.BackupDir, backup.ArchivePrefix) == 1
	}, "the scheduled backup never produced an archive")

	// The window advanced, so the next tick does not immediately fire again.
	if !s.nextBackupAt.After(time.Now().UTC()) {
		t.Errorf("nextBackupAt = %v, want it advanced past now", s.nextBackupAt)
	}
	waitForNotRunning(t, s)
}

// TestScheduledBackupPrunesToRetention asserts retention is applied after a
// dump, so the volume cannot fill up.
func TestScheduledBackupPrunesToRetention(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "* * * * *", 2)
	s := New(svc, "leader")

	if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Four older archives, named so a lexicographic sort is chronological.
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("%s-2026090%dT120000Z%s", backup.ArchivePrefix, i+1, backup.ArchiveExt)
		if err := os.WriteFile(filepath.Join(cfg.BackupDir, name), []byte("old"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	s.nextBackupAt = time.Now().UTC().Add(-time.Second)
	s.tick(context.Background())

	// The dump adds one, then retention trims to 2.
	waitFor(t, 10*time.Second, func() bool {
		return archiveCount(t, cfg.BackupDir, backup.ArchivePrefix) == 2
	}, "retention did not trim to 2 archives")
	waitForNotRunning(t, s)
}

// TestScheduledBackupSkipsWhenAlreadyRunning asserts a slow dump cannot be
// started twice by a following tick.
func TestScheduledBackupSkipsWhenAlreadyRunning(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "* * * * *", 7)
	s := New(svc, "leader")

	// Pretend a dump is in flight.
	s.backupRunning.Store(true)
	s.nextBackupAt = time.Now().UTC().Add(-time.Second)
	s.tick(context.Background())

	// The window still advances -- a skipped window is not retried forever --
	// but no dump starts.
	if !s.nextBackupAt.After(time.Now().UTC()) {
		t.Error("nextBackupAt did not advance on a skipped window")
	}
	time.Sleep(150 * time.Millisecond)
	if n := archiveCount(t, cfg.BackupDir, backup.ArchivePrefix); n != 0 {
		t.Errorf("%d archives written while a backup was already running", n)
	}
	s.backupRunning.Store(false)
}

// TestScheduledBackupDoesNotBlockTick asserts the dump runs off the tick, so a
// slow backup cannot delay recurrence generation or cache cleanup.
func TestScheduledBackupDoesNotBlockTick(t *testing.T) {
	svc, _ := newBackupSchedService(t, "* * * * *", 7)
	s := New(svc, "leader")
	s.nextBackupAt = time.Now().UTC().Add(-time.Second)

	start := time.Now()
	s.tick(context.Background())
	elapsed := time.Since(start)

	// The dump itself takes appreciably longer than this; if tick() waited for
	// it, this bound would not hold.
	if elapsed > 2*time.Second {
		t.Errorf("tick took %v, so it is waiting on the backup", elapsed)
	}
	waitForNotRunning(t, s)
}

// TestScheduledBackupNotLeaderDoesNothing asserts a replica that loses the
// lock writes nothing, so two instances sharing a database do not both dump.
func TestScheduledBackupNotLeaderDoesNothing(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "* * * * *", 7)

	// Another holder takes the backup lock first, with a long TTL.
	ok, err := svc.Store.AcquireLock(context.Background(), backupLockID, "someone-else", time.Hour)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if !ok {
		t.Fatal("could not seed the lock")
	}

	s := New(svc, "loser")
	s.runBackup(context.Background())

	if n := archiveCount(t, cfg.BackupDir, backup.ArchivePrefix); n != 0 {
		t.Errorf("a non-leader wrote %d archive(s)", n)
	}
}

// TestScheduledBackupHandlesDumpFailure asserts a failing dump is contained:
// it logs and returns rather than panicking or wedging the running flag.
func TestScheduledBackupHandlesDumpFailure(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "* * * * *", 7)
	// An empty secret makes the keyring derivation fail inside the dump.
	cfg.SessionSecret = ""

	s := New(svc, "leader")
	s.runBackup(context.Background())

	if n := archiveCount(t, cfg.BackupDir, backup.ArchivePrefix); n != 0 {
		t.Errorf("a failed dump left %d archive(s) behind", n)
	}
}

// TestMaybeBackupNotYetDue asserts a window in the future does nothing.
func TestMaybeBackupNotYetDue(t *testing.T) {
	svc, cfg := newBackupSchedService(t, "0 4 * * *", 7)
	s := New(svc, "leader")
	s.nextBackupAt = time.Now().UTC().Add(time.Hour)

	s.maybeBackup(context.Background())
	time.Sleep(100 * time.Millisecond)

	if n := archiveCount(t, cfg.BackupDir, backup.ArchivePrefix); n != 0 {
		t.Errorf("%d archives written before the window", n)
	}
	if s.backupRunning.Load() {
		t.Error("backupRunning was set for a window that had not arrived")
	}
}

// waitFor polls cond until it holds.
func waitFor(t *testing.T, limit time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// waitForNotRunning lets an in-flight dump finish, so its goroutine does not
// outlive the test and write into a cleaned-up temp directory.
func waitForNotRunning(t *testing.T, s *Scheduler) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !s.backupRunning.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("a scheduled backup was still running at the end of the test")
}
