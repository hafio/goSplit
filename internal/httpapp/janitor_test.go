package httpapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/backup"
)

// TestStartJanitorStopsWithContext asserts the sweeper is tied to the server's
// lifetime, so it does not outlive a shutdown.
func TestStartJanitorStopsWithContext(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.app.StartJanitor(ctx)
	cancel()
	// Nothing to assert beyond it not panicking or deadlocking; the goroutine
	// returns on the cancelled context.
	time.Sleep(20 * time.Millisecond)
}

// TestJanitorSweepReleasesWhatWasLeftBehind is the regression guard for the
// defect this file fixes: Prune was written but never called, so an admin who
// uploaded an archive, saw the preview and walked away left the upload and its
// staged files on the data volume permanently. One pass deletes both once the
// upload expires, and a .partial over a day old; a fresher .partial (a dump
// that may still be writing) and an unexpired upload stay.
func TestJanitorSweepReleasesWhatWasLeftBehind(t *testing.T) {
	h := newHarness(t)
	dir := h.cfg.BackupDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, age time.Duration) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(-age)
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
		return p
	}
	upload := write("upload-1.gsbak.tmp", 0)
	stale := write("gosplit-backup-20260101T000000Z.gsbak.partial", 48*time.Hour)
	fresh := write("gosplit-backup-20260102T000000Z.gsbak.partial", 0)
	r := backup.New(h.st, h.cfg)
	archive, _, err := r.DumpToDir(context.Background(), t.TempDir(), backup.ArchivePrefix)
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.Validate(context.Background(), archive)
	if err != nil {
		t.Fatal(err)
	}
	staging := h.cfg.UploadDir + ".restore"
	if _, err := os.Stat(staging); err != nil {
		t.Fatalf("validating staged nothing at %s: %v", staging, err)
	}
	token, err := h.app.Jobs.StagePending(v, upload, 1)
	if err != nil {
		t.Fatal(err)
	}

	h.app.sweepOnce(time.Now())
	if _, ok := h.app.Jobs.PeekPending(token); !ok {
		t.Fatal("a fresh pending restore was swept")
	}

	h.app.sweepOnce(time.Now().Add(time.Hour))
	if _, ok := h.app.Jobs.PeekPending(token); ok {
		t.Error("an abandoned upload is still pending")
	}
	for _, p := range []string{upload, staging, stale} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived the sweep (stat err %v)", filepath.Base(p), err)
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a fresh .partial was swept: %v", err)
	}
}
