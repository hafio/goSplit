package backup

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// mustBePOSIXPermissions skips a test that depends on mode bits actually
// restricting the owner. On Windows they do not: Go reports 0666 whatever was
// requested, and an owner can write to a directory it created regardless of
// mode -- so a 0500 fixture would not fail and the test would assert nothing.
// The Linux container is where this matters.
func mustBePOSIXPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode bits do not restrict the owner on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses mode bits, so an unwritable directory cannot be simulated")
	}
}

// TestEnsureWritableDirCreatesMissing asserts a missing path is created, so a
// first backup into a fresh directory works without the operator making it.
func TestEnsureWritableDirCreatesMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "backups")

	if err := EnsureWritableDir(dir, "BACKUP_DIR"); err != nil {
		t.Fatalf("EnsureWritableDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

// TestEnsureWritableDirAcceptsWritable asserts the happy path passes and leaves
// nothing behind -- a probe file left in BACKUP_DIR would show up in the admin
// page's archive listing.
func TestEnsureWritableDirAcceptsWritable(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureWritableDir(dir, "BACKUP_DIR"); err != nil {
		t.Fatalf("EnsureWritableDir: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("probe left %v behind", names)
	}
}

// TestEnsureWritableDirRejectsEmpty covers the unconfigured case, naming the
// setting rather than failing later on an empty path.
func TestEnsureWritableDirRejectsEmpty(t *testing.T) {
	err := EnsureWritableDir("", "BACKUP_DIR")
	if err == nil {
		t.Fatal("an empty directory was accepted")
	}
	if !strings.Contains(err.Error(), "BACKUP_DIR") {
		t.Errorf("error should name the setting: %v", err)
	}
}

// TestEnsureWritableDirReportsUnwritable is the reason this helper exists. The
// bare OS error is "permission denied" on a file path, which does not say who
// the process is or what to change -- and that cost real debugging time on a
// root-owned bind mount.
func TestEnsureWritableDirReportsUnwritable(t *testing.T) {
	mustBePOSIXPermissions(t)

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Restore write permission so the test's own cleanup can remove it.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := EnsureWritableDir(dir, "BACKUP_DIR")
	if err == nil {
		t.Fatal("an unwritable directory was accepted")
	}
	msg := err.Error()
	for _, want := range []string{
		dir,                       // which directory
		"BACKUP_DIR",              // which setting produced it
		"not writable",            // what failed
		strconv.Itoa(os.Getuid()), // who the process is
		"chown",                   // what to do next
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error is missing %q:\n %s", want, msg)
		}
	}
}

// TestEnsureWritableDirReportsUnwritableParent asserts the parent case is
// reported separately. It needs a different chown target, so collapsing the two
// would send the operator at the wrong directory.
func TestEnsureWritableDirReportsUnwritableParent(t *testing.T) {
	mustBePOSIXPermissions(t)

	parent := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(parent, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	child := filepath.Join(parent, "backups")
	err := EnsureWritableDir(child, "BACKUP_DIR")
	if err == nil {
		t.Fatal("a directory under an unwritable parent was accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cannot create") {
		t.Errorf("error should say it could not create the directory:\n %s", msg)
	}
	// The chown target must be the parent, not the directory that does not
	// exist yet.
	if !strings.Contains(msg, parent) {
		t.Errorf("error should name the parent %q as the chown target:\n %s", parent, msg)
	}
}

// TestEnsureWritableDirNamesEachSetting asserts the hint reaches the message
// for every call site, since three different settings can produce a path and
// the operator has to know which one to change.
func TestEnsureWritableDirNamesEachSetting(t *testing.T) {
	mustBePOSIXPermissions(t)

	for _, setting := range []string{"BACKUP_DIR", "BACKUP_DIR or -o", "AUTO_RESTORE_DIR"} {
		t.Run(setting, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "readonly")
			if err := os.Mkdir(dir, 0o500); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

			err := EnsureWritableDir(dir, setting)
			if err == nil {
				t.Fatal("an unwritable directory was accepted")
			}
			if !strings.Contains(err.Error(), setting) {
				t.Errorf("error does not name %q:\n %s", setting, err)
			}
		})
	}
}

// TestDumpToDirFailsFastOnUnwritableDir asserts the check runs BEFORE the dump,
// not after. Dumping a whole database and only then failing on the final write
// wastes the work and, on SQLite, holds a read snapshot for the duration for
// nothing.
func TestDumpToDirFailsFastOnUnwritableDir(t *testing.T) {
	mustBePOSIXPermissions(t)

	r, st, _ := newTestRunner(t)
	seedEverything(t, st)

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, _, err := r.DumpToDir(context.Background(), dir, ArchivePrefix)
	if err == nil {
		t.Fatal("dumping into an unwritable directory succeeded")
	}
	if !strings.Contains(err.Error(), "chown") {
		t.Errorf("error should carry the actionable remedy: %v", err)
	}

	// Nothing was written, not even a .partial.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod for inspection: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed dump left %d file(s) behind", len(entries))
	}
}

// TestProbeWritableLeavesNothing asserts the probe cleans up after itself even
// on repeated calls, since it runs on every dump and every auto-restore boot.
func TestProbeWritableLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := probeWritable(dir); err != nil {
			t.Fatalf("probeWritable: %v", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d probe file(s) left behind", len(entries))
	}
}

// TestProbeWritableFailsOnMissingDir asserts the probe reports a missing
// directory rather than silently passing.
func TestProbeWritableFailsOnMissingDir(t *testing.T) {
	if err := probeWritable(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("probing a missing directory succeeded")
	}
}
