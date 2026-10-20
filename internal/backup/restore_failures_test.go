package backup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestApplyRollsBackAMidRestoreFailure: a restore that fails after it has
// emptied the tables -- here an insert refused by a trigger -- leaves every
// table and the live uploads exactly as they were. The same validated archive
// then applies cleanly, so the failure left the connection and the staging
// usable.
func TestApplyRollsBackAMidRestoreFailure(t *testing.T) {
	ctx := context.Background()
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)
	archive, _, err := r.DumpToDir(ctx, cfg.BackupDir, ArchivePrefix)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	addCarol(t, st)
	stray := filepath.Join(cfg.UploadDir, "stray.png")
	if err := os.WriteFile(stray, []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := r.Validate(ctx, archive)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer v.Discard()
	before := fmt.Sprint(snapshotTables(t, st))

	exec(t, st, `CREATE TRIGGER fail_users BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'refused'); END`)
	if _, err := r.Apply(ctx, v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err == nil {
		t.Fatal("a restore whose inserts fail was reported as a success")
	}
	if fmt.Sprint(snapshotTables(t, st)) != before {
		t.Fatal("a failed restore left the database changed")
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("a failed restore touched the live uploads: %v", err)
	}

	exec(t, st, `DROP TRIGGER fail_users`)
	if _, err := r.Apply(ctx, v, ApplyOptions{Confirmed: true, SkipSafetyDump: true}); err != nil {
		t.Fatalf("re-applying after the failure: %v", err)
	}
	if n := countRows(t, st, "users"); n != 2 {
		t.Errorf("users after the retried restore = %d, want the archive's 2", n)
	}
}

// TestApplyNeedsItsSafetyDump: when the pre-restore safety dump -- the
// operator's only way back -- cannot be written, the restore is not attempted.
func TestApplyNeedsItsSafetyDump(t *testing.T) {
	ctx := context.Background()
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)
	archive, _, err := r.DumpToDir(ctx, cfg.BackupDir, ArchivePrefix)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	addCarol(t, st)
	v, err := r.Validate(ctx, archive)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer v.Discard()
	before := fmt.Sprint(snapshotTables(t, st))
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = r.Apply(ctx, v, ApplyOptions{Confirmed: true, SafetyDumpDir: notADir})
	if err == nil || !strings.Contains(err.Error(), "safety dump failed") || !strings.Contains(err.Error(), "not attempted") {
		t.Fatalf("Apply = %v, want the safety-dump refusal", err)
	}
	if fmt.Sprint(snapshotTables(t, st)) != before {
		t.Error("the restore went ahead without its safety dump")
	}
	if _, err := os.Stat(v.staging); err != nil {
		t.Errorf("the refused restore lost its staged uploads: %v", err)
	}
}

// TestRoundTripAcrossBatchesAndParts: a table over 1,000 rows is inserted in
// several batches, and one over 8 MiB is split into several archive parts.
// Every fixture elsewhere stays far below both, so this is the only test of a
// realistically sized database: nothing may be lost or repeated at a boundary.
func TestRoundTripAcrossBatchesAndParts(t *testing.T) {
	ctx := context.Background()
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	exec(t, st, `WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 2100)
		INSERT INTO expense_notes (id, expense_id, user_id, note, created_at)
		SELECT 100 + n, 'exp-1', 1, 'note ' || n, '2026-01-01T00:00:00.000Z' FROM seq`)
	for i := 0; i < 10; i++ {
		exec(t, st, `INSERT INTO expense_notes (id, expense_id, user_id, note, created_at) VALUES (?, 'exp-1', 1, ?, '2026-01-02T00:00:00.000Z')`,
			5000+i, fmt.Sprintf("%d:", i)+strings.Repeat("n", 900_000))
	}
	want := countRows(t, st, "expense_notes")
	before := fmt.Sprint(snapshotTables(t, st))

	archive, _, err := r.DumpToDir(ctx, cfg.BackupDir, ArchivePrefix)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	wipeAll(t, st)
	v, err := r.Validate(ctx, archive)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if parts := len(v.Index.TableParts["expense_notes"]); parts < 2 {
		t.Fatalf("expense_notes went into %d part(s); the test needs at least 2", parts)
	}
	rep, err := r.Apply(ctx, v, ApplyOptions{Confirmed: true, SkipSafetyDump: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fmt.Sprint(snapshotTables(t, st)) != before {
		t.Error("the restored data differs from what was dumped")
	}
	if rep.RowsRestored["expense_notes"] != want {
		t.Errorf("restored %d expense_notes rows, want %d", rep.RowsRestored["expense_notes"], want)
	}
}

// TestCancelledDumpLeavesNothing: a backup cancelled mid-way (Ctrl-C, or a
// server shutting down during a scheduled backup) reports the cancellation and
// leaves no half-written file behind.
func TestCancelledDumpLeavesNothing(t *testing.T) {
	r, st, _ := newTestRunner(t)
	seedEverything(t, st)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()

	if _, _, err := r.DumpToDir(ctx, dir, ArchivePrefix); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled dump = %v, want context.Canceled", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a cancelled dump left files behind: %v", entries)
	}
}

// TestDumpFailsOnAnUnreadableUpload: an upload the process cannot read -- a
// file or a directory with the wrong owner -- fails the backup rather than
// producing an archive that silently lacks it.
func TestDumpFailsOnAnUnreadableUpload(t *testing.T) {
	mustBePOSIXPermissions(t)
	for _, rel := range []string{"nested/r.pdf", "nested/deep"} {
		t.Run(rel, func(t *testing.T) {
			r, st, cfg := newTestRunner(t)
			seedEverything(t, st)
			seedUploads(t, cfg)
			p := filepath.Join(cfg.UploadDir, filepath.FromSlash(rel))
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, info.Mode().Perm()) })

			_, err = r.Dump(context.Background(), io.Discard)
			if err == nil || !strings.Contains(err.Error(), filepath.Base(p)) {
				t.Errorf("dump with %s unreadable = %v, want an error naming it", rel, err)
			}
		})
	}
}

// TestPruneDeletesExpiredUploads: an upload that was never confirmed is
// deleted from disk with its staged files once it expires -- each can be as
// large as RESTORE_MAX_UPLOAD_MB -- and one still within its time is left alone.
func TestPruneDeletesExpiredUploads(t *testing.T) {
	tr := NewJobTracker()
	dir := t.TempDir()
	upload, staging := filepath.Join(dir, "upload.gsbak.tmp"), filepath.Join(dir, "staging")
	if err := os.WriteFile(upload, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	token, err := tr.StagePending(&Validated{staging: staging}, upload, 1)
	if err != nil {
		t.Fatal(err)
	}

	tr.Prune(time.Now())
	if _, ok := tr.PeekPending(token); !ok {
		t.Fatal("a fresh upload was pruned")
	}
	tr.Prune(time.Now().Add(2 * jobTTL))
	if _, ok := tr.PeekPending(token); ok {
		t.Error("an expired upload is still pending")
	}
	for _, p := range []string{upload, staging} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived the prune (stat err %v)", p, err)
		}
	}
}

// TestScanEnforcesTheTablePartCap: a table part over RESTORE_MAX_TABLE_FILE_MB
// is refused and named, before its data is used.
func TestScanEnforcesTheTablePartCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	entries := baselineEntries()
	for i := range entries {
		if entries[i].name == tablePartName("users", 0) {
			entries[i].body = bytes.Repeat([]byte("x"), 4096)
		}
	}
	archive := craftArchive(t, k, entries, baselineManifest())
	hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
	limits := Limits{MaxTotalBytes: 1 << 28, MaxUploadFileBytes: 1 << 20, MaxTablePartBytes: 1024, MaxEntries: 1000, MaxJSONLLineBytes: 1 << 16}
	_, err = Scan(opener, k, hdr, aad, limits, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "RESTORE_MAX_TABLE_FILE_MB") {
		t.Errorf("an oversized table part = %v, want a refusal naming RESTORE_MAX_TABLE_FILE_MB", err)
	}
}

// TestScanEnforcesTheManifestCap: a manifest over the 1 MiB cap is refused
// instead of being read into memory whole.
func TestScanEnforcesTheManifestCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	man := baselineManifest()
	man.GosplitVersion = strings.Repeat("v", maxHeaderLen+1)
	archive := craftArchive(t, k, baselineEntries(), man)
	if _, err := scanCrafted(t, archive, t.TempDir()); err == nil || !strings.Contains(err.Error(), ManifestEntry) {
		t.Errorf("an oversized manifest = %v, want a refusal naming %s", err, ManifestEntry)
	}
}

// TestExtractUploadStaysInsideStaging: an upload path that would land outside
// the staging directory is refused and nothing is written. Scan's path checks
// stop such names first; this is the check on the joined path itself.
func TestExtractUploadStaysInsideStaging(t *testing.T) {
	parent := t.TempDir()
	staging := filepath.Join(parent, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := extractUpload(strings.NewReader("x"), staging, "../escape.png", 64); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("an escaping upload path = %v, want a refusal", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "escape.png")); !os.IsNotExist(err) {
		t.Errorf("the escaping upload was written (stat err %v)", err)
	}
}

// TestReadHeaderRefusesAFutureFormat: an archive written by a newer build --
// a format version this one does not know -- is refused with a message saying
// to restore it with the build that wrote it.
func TestReadHeaderRefusesAFutureFormat(t *testing.T) {
	archive := craftArchive(t, testKeyring(t, testSecret), baselineEntries(), baselineManifest())
	archive[8] = byte(FormatVersion) + 1
	if _, _, _, err := ReadHeader(bytes.NewReader(archive)); err == nil || !strings.Contains(err.Error(), "the build that wrote it") {
		t.Errorf("a future-format archive = %v, want a refusal", err)
	}
}
