package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// writeArchiveFiles creates n dummy archives with ascending timestamps.
func writeArchiveFiles(t *testing.T, dir, prefix string, n int) []string {
	t.Helper()
	var names []string
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s-2026090%dT120000Z%s", prefix, i, ArchiveExt)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		names = append(names, name)
	}
	return names
}

func lsNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// TestPruneKeepsNewest asserts retention keeps the newest N. Archive names
// carry a fixed-width UTC timestamp, so lexicographic order is chronological.
func TestPruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	all := writeArchiveFiles(t, dir, ArchivePrefix, 5)

	removed, err := PruneOldArchives(dir, ArchivePrefix, 2)
	if err != nil {
		t.Fatalf("PruneOldArchives: %v", err)
	}
	if removed != 3 {
		t.Errorf("removed %d, want 3", removed)
	}
	got := lsNames(t, dir)
	want := []string{all[3], all[4]}
	sort.Strings(want)
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("kept %v, want the two newest %v", got, want)
	}
}

// TestPruneNoopWhenUnderRetention asserts nothing is removed when the count is
// already within the limit.
func TestPruneNoopWhenUnderRetention(t *testing.T) {
	dir := t.TempDir()
	writeArchiveFiles(t, dir, ArchivePrefix, 2)

	removed, err := PruneOldArchives(dir, ArchivePrefix, 7)
	if err != nil {
		t.Fatalf("PruneOldArchives: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed %d, want 0", removed)
	}
	if n := len(lsNames(t, dir)); n != 2 {
		t.Errorf("%d files remain, want 2", n)
	}
}

// TestPruneKeepsEverythingWhenDisabled asserts a non-positive count means
// unlimited retention, rather than deleting everything.
func TestPruneKeepsEverythingWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	writeArchiveFiles(t, dir, ArchivePrefix, 4)

	for _, keep := range []int{0, -1} {
		removed, err := PruneOldArchives(dir, ArchivePrefix, keep)
		if err != nil {
			t.Fatalf("keep=%d: %v", keep, err)
		}
		if removed != 0 {
			t.Errorf("keep=%d removed %d files; a non-positive count must keep everything", keep, removed)
		}
	}
	if n := len(lsNames(t, dir)); n != 4 {
		t.Errorf("%d files remain, want 4", n)
	}
}

// TestPruneIgnoresOtherPrefixes is the important isolation property: pruning
// scheduled backups must never touch pre-restore safety dumps, which are the
// operator's way back from a bad restore.
func TestPruneIgnoresOtherPrefixes(t *testing.T) {
	dir := t.TempDir()
	writeArchiveFiles(t, dir, ArchivePrefix, 4)
	safety := writeArchiveFiles(t, dir, SafetyDumpPrefix, 3)

	if _, err := PruneOldArchives(dir, ArchivePrefix, 1); err != nil {
		t.Fatalf("PruneOldArchives: %v", err)
	}
	for _, name := range safety {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("pruning backups removed the safety dump %s", name)
		}
	}
}

// TestPruneIgnoresUnrelatedFiles asserts the sweep is narrow: it must not
// remove the database, an upload, or a partial file.
func TestPruneIgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	writeArchiveFiles(t, dir, ArchivePrefix, 3)
	keep := []string{
		"gosplit.db",
		"notes.txt",
		ArchivePrefix + "-20260901T120000Z" + ArchiveExt + ".partial",
		"other-20260901T120000Z" + ArchiveExt,
	}
	for _, n := range keep {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	if _, err := PruneOldArchives(dir, ArchivePrefix, 1); err != nil {
		t.Fatalf("PruneOldArchives: %v", err)
	}
	for _, n := range keep {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("prune removed unrelated file %s", n)
		}
	}
}

// TestPruneMissingDirIsNotAnError asserts a not-yet-created backup directory
// is simply nothing to do.
func TestPruneMissingDirIsNotAnError(t *testing.T) {
	removed, err := PruneOldArchives(filepath.Join(t.TempDir(), "absent"), ArchivePrefix, 1)
	if err != nil {
		t.Errorf("a missing directory should not be an error: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed %d from a missing directory", removed)
	}
	if _, err := PruneOldArchives("", ArchivePrefix, 1); err != nil {
		t.Errorf("an empty directory setting should be a no-op: %v", err)
	}
}

// TestSweepStalePartials asserts only old partials go.
func TestSweepStalePartials(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	stale := filepath.Join(dir, "gosplit-backup-20260901T120000Z"+ArchiveExt+partialSuffix)
	fresh := filepath.Join(dir, "gosplit-backup-20260902T120000Z"+ArchiveExt+partialSuffix)
	real := filepath.Join(dir, "gosplit-backup-20260903T120000Z"+ArchiveExt)
	for _, p := range []string{stale, fresh, real} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	old := now.Add(-2 * stalePartialAfter)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if n := SweepStalePartials(dir, now); n != 1 {
		t.Errorf("swept %d, want 1", n)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("the stale partial survived")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("a fresh partial was swept; a dump in flight would be broken")
	}
	if _, err := os.Stat(real); err != nil {
		t.Error("a completed archive was swept")
	}
}
