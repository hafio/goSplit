package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// addCarol adds a third user, so a restore of the two-user seed is visible.
func addCarol(t *testing.T, st *store.Store) {
	t.Helper()
	exec(t, st, `INSERT INTO users (id, name, email, currency, default_currency,
		preferred_language, role, hidden_friend_ids, created_at, theme_color)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		3, "Carol", "carol@example.com", "GBP", "GBP", "en", "USER", "[]",
		"2026-04-01T00:00:00.000Z", "burgundy")
}

// autoRestoreFixture seeds an instance, writes its archive into a fresh
// AUTO_RESTORE_DIR, then makes the live data diverge: Carol joins and a stray
// upload appears. It returns the runner pieces, the directory and the seeded
// snapshot an auto-restore should bring back.
func autoRestoreFixture(t *testing.T) (*Runner, *store.Store, *config.Config, string, string) {
	t.Helper()
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)
	seeded := fmt.Sprint(snapshotTables(t, st))
	dir := t.TempDir()
	if _, _, err := r.DumpToDir(context.Background(), dir, ArchivePrefix); err != nil {
		t.Fatalf("dump: %v", err)
	}
	cfg.AutoRestoreDir = dir
	addCarol(t, st)
	if err := os.WriteFile(filepath.Join(cfg.UploadDir, "stray.png"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	return r, st, cfg, dir, seeded
}

// safetyDumps lists the pre-restore dumps auto-restore left in dir.
func safetyDumps(t *testing.T, dir string) []string {
	t.Helper()
	got, err := filepath.Glob(filepath.Join(dir, SafetyDumpPrefix+"*"+ArchiveExt))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestFindAutoRestoreArchive: exactly one archive is picked; its own safety
// dumps, half-written files, other files and directories are ignored; two
// archives refuse to guess; an empty or missing directory means nothing to do.
func TestFindAutoRestoreArchive(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.gsbak", "notes.txt", "a.gsbak.partial", SafetyDumpPrefix + "-20260101T000000Z.gsbak"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "z.gsbak"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := findAutoRestoreArchive(dir); err != nil || got != filepath.Join(dir, "a.gsbak") {
		t.Errorf("one archive among the noise = %q, %v; want a.gsbak", got, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "b.gsbak"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findAutoRestoreArchive(dir); err == nil || got != "" || !strings.Contains(err.Error(), "a.gsbak, b.gsbak") || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("two archives = %q, %v; want an ambiguity error naming both", got, err)
	}

	for name, d := range map[string]string{"empty": t.TempDir(), "missing": filepath.Join(t.TempDir(), "nope")} {
		if got, err := findAutoRestoreArchive(d); err != nil || got != "" {
			t.Errorf("%s directory = %q, %v; want nothing to do", name, got, err)
		}
	}

	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findAutoRestoreArchive(notADir); err == nil || got != "" {
		t.Errorf("a file as the directory = %q, %v; want an error", got, err)
	}
}

// TestCheckAutoRestoreRestoresOnceAndRecordsIt: the first boot replaces the
// data and the uploads with the archive's, writes a safety dump that can itself
// be restored, leaves the archive in place and records the marker. The next
// boot does nothing, so data written since survives.
func TestCheckAutoRestoreRestoresOnceAndRecordsIt(t *testing.T) {
	ctx := context.Background()
	r, st, cfg, dir, seeded := autoRestoreFixture(t)

	if err := CheckAutoRestore(ctx, st, cfg); err != nil {
		t.Fatalf("auto-restore: %v", err)
	}
	if got := fmt.Sprint(snapshotTables(t, st)); got != seeded {
		t.Error("the database does not match the archive after auto-restore")
	}
	if _, err := os.Stat(filepath.Join(cfg.UploadDir, "stray.png")); !os.IsNotExist(err) {
		t.Errorf("the uploads were not replaced (stray.png stat err %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerName)); err != nil {
		t.Errorf("no marker after a successful restore: %v", err)
	}
	dumps := safetyDumps(t, dir)
	if len(dumps) != 1 {
		t.Fatalf("safety dumps in AUTO_RESTORE_DIR = %v, want exactly one", dumps)
	}
	v, err := r.Validate(ctx, dumps[0])
	if err != nil {
		t.Fatalf("the safety dump cannot be restored: %v", err)
	}
	v.Discard()
	if archives, _ := filepath.Glob(filepath.Join(dir, ArchivePrefix+"*"+ArchiveExt)); len(archives) != 1 {
		t.Errorf("the archive was not left in place: %v", archives)
	}

	addCarol(t, st)
	if err := CheckAutoRestore(ctx, st, cfg); err != nil {
		t.Fatalf("second boot: %v", err)
	}
	if n := countRows(t, st, "users"); n != 3 {
		t.Errorf("the second boot restored again: %d users, want Carol kept (3)", n)
	}
}

// TestCheckAutoRestoreLeavesItToTheLockHolder: while another replica holds the
// restore lock this one boots without touching anything. Once that lock
// expires, the restore runs -- so the first skip was the lock and nothing else.
func TestCheckAutoRestoreLeavesItToTheLockHolder(t *testing.T) {
	ctx := context.Background()
	_, st, cfg, dir, seeded := autoRestoreFixture(t)
	if leader, err := st.AcquireLock(ctx, autoRestoreLockID, "other-replica", time.Hour); err != nil || !leader {
		t.Fatalf("take the lock as another replica: %v %v", leader, err)
	}
	before := fmt.Sprint(snapshotTables(t, st))

	if err := CheckAutoRestore(ctx, st, cfg); err != nil {
		t.Fatalf("auto-restore while locked: %v", err)
	}
	if fmt.Sprint(snapshotTables(t, st)) != before {
		t.Error("the database changed while another replica held the lock")
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerName)); !os.IsNotExist(err) {
		t.Errorf("a skipped restore wrote the marker (stat err %v)", err)
	}
	if dumps := safetyDumps(t, dir); len(dumps) != 0 {
		t.Errorf("a skipped restore wrote a safety dump: %v", dumps)
	}

	exec(t, st, `UPDATE scheduler_locks SET expires_at = '2000-01-01T00:00:00.000Z' WHERE id = ?`, autoRestoreLockID)
	if err := CheckAutoRestore(ctx, st, cfg); err != nil {
		t.Fatalf("auto-restore after the lock expired: %v", err)
	}
	if fmt.Sprint(snapshotTables(t, st)) != seeded {
		t.Error("the restore did not run once the lock had expired")
	}
}

// TestCheckAutoRestoreFailuresAbortTheBoot: an archive that cannot be
// restored -- unreadable, or failing halfway through the restore -- stops the
// boot with a message naming the file and AUTO_RESTORE_DIR, leaves the
// database as it was and writes no marker, so nothing is half-recorded.
func TestCheckAutoRestoreFailuresAbortTheBoot(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, st *store.Store, dir string) string
	}{
		{"an unreadable archive", func(t *testing.T, st *store.Store, dir string) string {
			for _, a := range mustGlob(t, filepath.Join(dir, "*"+ArchiveExt)) {
				if err := os.Remove(a); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "garbage.gsbak"), []byte("not an archive"), 0o644); err != nil {
				t.Fatal(err)
			}
			return "garbage.gsbak"
		}},
		{"a restore that fails halfway", func(t *testing.T, st *store.Store, dir string) string {
			exec(t, st, `CREATE TRIGGER fail_users BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'refused'); END`)
			return filepath.Base(mustGlob(t, filepath.Join(dir, ArchivePrefix+"*"+ArchiveExt))[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, st, cfg, dir, _ := autoRestoreFixture(t)
			name := tc.setup(t, st, dir)
			before := withoutLocks(t, st)

			err := CheckAutoRestore(context.Background(), st, cfg)
			if err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "AUTO_RESTORE_DIR") {
				t.Fatalf("auto-restore = %v, want a failure naming %s and AUTO_RESTORE_DIR", err, name)
			}
			if withoutLocks(t, st) != before {
				t.Error("a failed auto-restore changed the database")
			}
			if _, err := os.Stat(filepath.Join(dir, MarkerName)); !os.IsNotExist(err) {
				t.Errorf("a failed auto-restore wrote the marker (stat err %v)", err)
			}
		})
	}
}

// TestCheckAutoRestoreWithoutAnArchiveDoesNothing: a configured but empty
// directory is the normal state after setup; boot carries on, nothing changes
// and nothing is written there -- a marker would block a later archive.
func TestCheckAutoRestoreWithoutAnArchiveDoesNothing(t *testing.T) {
	_, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	cfg.AutoRestoreDir = t.TempDir()
	before := fmt.Sprint(snapshotTables(t, st))

	if err := CheckAutoRestore(context.Background(), st, cfg); err != nil {
		t.Fatalf("auto-restore with an empty directory: %v", err)
	}
	if fmt.Sprint(snapshotTables(t, st)) != before {
		t.Error("an empty AUTO_RESTORE_DIR changed the database")
	}
	if entries, _ := os.ReadDir(cfg.AutoRestoreDir); len(entries) != 0 {
		t.Errorf("files appeared in an empty AUTO_RESTORE_DIR: %v", entries)
	}
}

// withoutLocks is the table snapshot minus scheduler_locks. The auto-restore
// lock is taken outside the restore transaction and lapses on its own TTL, so a
// failed attempt leaves that one row behind.
func withoutLocks(t *testing.T, st *store.Store) string {
	t.Helper()
	snap := snapshotTables(t, st)
	delete(snap, "scheduler_locks")
	return fmt.Sprint(snap)
}

func mustGlob(t *testing.T, pattern string) []string {
	t.Helper()
	got, err := filepath.Glob(pattern)
	if err != nil || len(got) == 0 {
		t.Fatalf("glob %s = %v, %v", pattern, got, err)
	}
	return got
}
