package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

// swapFixture builds an upload directory plus whichever of the staging and
// .old siblings a test needs, so each crash point can be reproduced exactly.
func swapFixture(t *testing.T, live, staging, old bool) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		UploadDir: filepath.Join(dir, "uploads"),
		BackupDir: filepath.Join(dir, "backups"),
	}
	mk := func(path, marker string) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(path, "which.txt"), []byte(marker), 0o644); err != nil {
			t.Fatalf("write marker in %s: %v", path, err)
		}
	}
	if live {
		mk(cfg.UploadDir, "live")
	}
	if staging {
		mk(stagingDirFor(cfg.UploadDir), "staging")
	}
	if old {
		mk(oldDirFor(cfg.UploadDir), "old")
	}
	return cfg
}

func marker(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "which.txt"))
	if err != nil {
		t.Fatalf("read marker in %s: %v", dir, err)
	}
	return string(b)
}

// TestRecoverNothingToDo asserts the ordinary case is a no-op.
func TestRecoverNothingToDo(t *testing.T) {
	cfg := swapFixture(t, true, false, false)
	if err := RecoverIncompleteSwap(cfg); err != nil {
		t.Fatalf("RecoverIncompleteSwap: %v", err)
	}
	if got := marker(t, cfg.UploadDir); got != "live" {
		t.Errorf("upload directory changed: %q", got)
	}
}

// TestRecoverPreCommitCrash covers a crash before the transaction committed,
// or a validated restore that was never confirmed: the staged uploads are
// irrelevant and the database is untouched, so the staging goes.
func TestRecoverPreCommitCrash(t *testing.T) {
	cfg := swapFixture(t, true, true, false)
	if err := RecoverIncompleteSwap(cfg); err != nil {
		t.Fatalf("RecoverIncompleteSwap: %v", err)
	}
	if _, err := os.Stat(stagingDirFor(cfg.UploadDir)); !os.IsNotExist(err) {
		t.Error("stale staging directory survived")
	}
	if got := marker(t, cfg.UploadDir); got != "live" {
		t.Errorf("live uploads were disturbed: %q", got)
	}
}

// TestRecoverCleanupOnlyCrash covers a crash after both renames succeeded but
// before the leftover was removed. The swap is complete, so only the leftover
// goes -- and this must NOT be reported as an inconsistency.
func TestRecoverCleanupOnlyCrash(t *testing.T) {
	cfg := swapFixture(t, true, false, true)
	if err := RecoverIncompleteSwap(cfg); err != nil {
		t.Fatalf("a completed swap awaiting cleanup should not error: %v", err)
	}
	if _, err := os.Stat(oldDirFor(cfg.UploadDir)); !os.IsNotExist(err) {
		t.Error("leftover .old directory survived")
	}
	if got := marker(t, cfg.UploadDir); got != "live" {
		t.Errorf("the restored uploads were replaced: %q", got)
	}
}

// TestRecoverMidSwapCrash is the dangerous one: the crash landed between the
// two renames, so there is no upload directory at all. The previous uploads go
// back, and the operator is told loudly, because the database may already hold
// the restored data.
func TestRecoverMidSwapCrash(t *testing.T) {
	cfg := swapFixture(t, false, true, true)

	err := RecoverIncompleteSwap(cfg)
	if err == nil {
		t.Fatal("an interrupted swap was not reported")
	}
	for _, want := range []string{"interrupted", "may disagree"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should explain the inconsistency (missing %q): %v", want, err)
		}
	}
	// The instance must still have an upload directory.
	if got := marker(t, cfg.UploadDir); got != "old" {
		t.Errorf("previous uploads were not moved back: %q", got)
	}
	if _, err := os.Stat(stagingDirFor(cfg.UploadDir)); !os.IsNotExist(err) {
		t.Error("staging directory was left behind")
	}
}

// TestRecoverMidSwapCrashWithoutStaging covers the same crash point when the
// staging directory is already gone.
func TestRecoverMidSwapCrashWithoutStaging(t *testing.T) {
	cfg := swapFixture(t, false, false, true)
	if err := RecoverIncompleteSwap(cfg); err == nil {
		t.Fatal("an interrupted swap was not reported")
	}
	if got := marker(t, cfg.UploadDir); got != "old" {
		t.Errorf("previous uploads were not moved back: %q", got)
	}
}

// TestRecoverNoUploadDirConfigured asserts an unset UPLOAD_DIR is a no-op
// rather than an error, so recovery never blocks a boot it has no stake in.
func TestRecoverNoUploadDirConfigured(t *testing.T) {
	if err := RecoverIncompleteSwap(&config.Config{}); err != nil {
		t.Errorf("an unset UPLOAD_DIR should be a no-op: %v", err)
	}
}

// TestRecoverIsIdempotent asserts running it twice is safe, which matters
// because both the server boot and the CLI restore call it.
func TestRecoverIsIdempotent(t *testing.T) {
	cfg := swapFixture(t, true, true, false)
	if err := RecoverIncompleteSwap(cfg); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := RecoverIncompleteSwap(cfg); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := marker(t, cfg.UploadDir); got != "live" {
		t.Errorf("uploads disturbed by a repeat run: %q", got)
	}
}

// TestStagingAndOldAreSiblings asserts the two directories sit beside the
// upload directory. That is what makes the swap two same-filesystem renames
// rather than a copy, and it is what lets recovery recognize them by name.
func TestStagingAndOldAreSiblings(t *testing.T) {
	for _, dir := range []string{"/data/uploads", `C:\data\uploads`, "./data/uploads/"} {
		staging, old := stagingDirFor(dir), oldDirFor(dir)
		if staging == dir || old == dir || staging == old {
			t.Errorf("%q: derived directories collide (%q, %q)", dir, staging, old)
		}
		if filepath.Dir(filepath.Clean(staging)) != filepath.Dir(filepath.Clean(strings.TrimRight(dir, `/\`))) {
			t.Errorf("%q: staging %q is not a sibling", dir, staging)
		}
	}
}
