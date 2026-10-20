package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestApplyRefusesChangedStaging: a restore whose staged uploads are not the
// ones its own validation extracted -- a file added, a file resized, or the
// whole directory gone, as another upload or a command-line restore leaves it
// -- is refused before anything is touched: no safety dump, no table changed,
// the live uploads as they were.
func TestApplyRefusesChangedStaging(t *testing.T) {
	ctx := context.Background()
	r, st, cfg := newTestRunner(t)
	seedEverything(t, st)
	seedUploads(t, cfg)
	archive, _, err := r.DumpToDir(ctx, cfg.BackupDir, ArchivePrefix)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	exec(t, st, `UPDATE users SET name = 'Changed since the backup' WHERE id = 1`)
	before := fmt.Sprint(snapshotTables(t, st))
	liveFile := filepath.Join(cfg.UploadDir, "a.png")
	live, err := os.ReadFile(liveFile)
	if err != nil {
		t.Fatal(err)
	}
	dumps := t.TempDir()

	for _, tc := range []struct {
		name   string
		mutate func(staging string) error
	}{
		{"a file added", func(s string) error { return os.WriteFile(filepath.Join(s, "extra.png"), []byte("x"), 0o644) }},
		{"a file resized", func(s string) error {
			return os.WriteFile(filepath.Join(s, "a.png"), []byte("a different length"), 0o644)
		}},
		{"the directory removed", os.RemoveAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := r.Validate(ctx, archive)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			defer v.Discard()
			if err := tc.mutate(v.staging); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Apply(ctx, v, ApplyOptions{Confirmed: true, SafetyDumpDir: dumps}); !errors.Is(err, ErrStagingChanged) {
				t.Fatalf("Apply = %v, want ErrStagingChanged", err)
			}
			if got := fmt.Sprint(snapshotTables(t, st)); got != before {
				t.Error("a refused restore changed the database")
			}
			if b, err := os.ReadFile(liveFile); err != nil || string(b) != string(live) {
				t.Errorf("a refused restore touched the live uploads: %q %v", b, err)
			}
			if entries, _ := os.ReadDir(dumps); len(entries) != 0 {
				t.Errorf("a refused restore wrote a safety dump: %v", entries)
			}
		})
	}
}
