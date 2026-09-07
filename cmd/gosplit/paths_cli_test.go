package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// pathsCfg builds a config whose directories live under one temp root.
func pathsCfg(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{
		Engine:      config.EngineSQLite,
		DatabaseURL: "file:" + filepath.Join(dir, "db", "gosplit.db"),
		UploadDir:   filepath.Join(dir, "uploads"),
		BackupDir:   filepath.Join(dir, "backups"),
	}
}

// TestRunPathsCLIRejectsArguments covers the argument guard.
func TestRunPathsCLIRejectsArguments(t *testing.T) {
	if err := runPathsCLI([]string{"unexpected"}); err == nil {
		t.Error("paths accepted a positional argument")
	}
}

// TestCollectDirStatusesCoversEverySetting asserts every directory the app
// writes to is reported. A missing one is the whole point of the command:
// distroless has no ls, so what this prints is all an operator can see.
func TestCollectDirStatusesCoversEverySetting(t *testing.T) {
	cfg := pathsCfg(t)
	cfg.AutoRestoreDir = filepath.Join(t.TempDir(), "restore")

	got := map[string]bool{}
	for _, d := range collectDirStatuses(cfg) {
		got[d.Setting] = true
	}
	for _, want := range []string{"DATABASE_URL", "UPLOAD_DIR", "BACKUP_DIR", "AUTO_RESTORE_DIR"} {
		if !got[want] {
			t.Errorf("collectDirStatuses does not report %s", want)
		}
	}
}

// TestCollectDirStatusesOmitsDatabaseDirForPostgres asserts a Postgres
// deployment is not told about a SQLite directory it does not have.
func TestCollectDirStatusesOmitsDatabaseDirForPostgres(t *testing.T) {
	cfg := pathsCfg(t)
	cfg.Engine = config.EnginePostgres
	cfg.DatabaseURL = "host=db port=5432 dbname=gosplit"

	for _, d := range collectDirStatuses(cfg) {
		if d.Setting == "DATABASE_URL" {
			t.Error("a Postgres config reported a SQLite data directory")
		}
	}
}

// TestCollectDirStatusesReportsUnsetAutoRestore asserts the unset case shows up
// as unset rather than vanishing, so an operator can tell the difference
// between "off" and "not reported".
func TestCollectDirStatusesReportsUnsetAutoRestore(t *testing.T) {
	cfg := pathsCfg(t)
	cfg.AutoRestoreDir = ""

	for _, d := range collectDirStatuses(cfg) {
		if d.Setting != "AUTO_RESTORE_DIR" {
			continue
		}
		if d.Path != "(not set)" {
			t.Errorf("unset AUTO_RESTORE_DIR reported as %q", d.Path)
		}
		if d.Exists || d.Writable {
			t.Error("unset AUTO_RESTORE_DIR reported as existing or writable")
		}
		return
	}
	t.Error("AUTO_RESTORE_DIR was not reported at all")
}

// TestInspectDirWritable asserts the happy path, and that the probe leaves
// nothing behind -- `paths` is a diagnostic and must not change what it reports.
func TestInspectDirWritable(t *testing.T) {
	dir := t.TempDir()

	d := inspectDir("BACKUP_DIR", dir)
	if !d.Exists || !d.Writable {
		t.Fatalf("a writable directory reported exists=%v writable=%v (%s)", d.Exists, d.Writable, d.Problem)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("inspectDir left %d file(s) behind", len(entries))
	}
}

// TestInspectDirMissing asserts a not-yet-created directory is reported as
// such rather than as an error, since the app creates it at boot.
func TestInspectDirMissing(t *testing.T) {
	d := inspectDir("BACKUP_DIR", filepath.Join(t.TempDir(), "absent"))
	if d.Exists {
		t.Error("a missing directory was reported as existing")
	}
	if !strings.Contains(d.Problem, "does not exist") {
		t.Errorf("problem = %q, want it to say the directory does not exist", d.Problem)
	}
}

// TestInspectDirNotADirectory asserts a file where a directory belongs is
// called out, which is otherwise a confusing failure later.
func TestInspectDirNotADirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	d := inspectDir("BACKUP_DIR", p)
	if d.Writable {
		t.Error("a plain file was reported as a writable directory")
	}
	if !strings.Contains(d.Problem, "not a directory") {
		t.Errorf("problem = %q", d.Problem)
	}
}

// TestInspectDirUnwritable is the case the command exists for: a root-owned
// bind mount. It must report the directory as present but not writable.
func TestInspectDirUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("mode bits do not restrict the owner here")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	d := inspectDir("BACKUP_DIR", dir)
	if !d.Exists {
		t.Error("an existing directory was reported as missing")
	}
	if d.Writable {
		t.Error("an unwritable directory was reported as writable")
	}
	if d.Owner == "" {
		t.Error("no owner reported, which is half the diagnosis on a bind mount")
	}
}

// TestInspectDirEmptyPath covers an unset setting.
func TestInspectDirEmptyPath(t *testing.T) {
	d := inspectDir("AUTO_RESTORE_DIR", "")
	if d.Path != "(not set)" || d.Exists {
		t.Errorf("empty path reported as %+v", d)
	}
}

// TestPrintDirContentsListsArchivesAndMarker asserts the listing surfaces the
// two things an operator is looking for -- the archives, and whether a startup
// restore has already run -- without dumping every file.
func TestPrintDirContentsListsArchivesAndMarker(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"gosplit-backup-20260907T020000Z" + backup.ArchiveExt,
		"gosplit-pre-restore-20260907T030000Z" + backup.ArchiveExt,
		backup.MarkerName,
		"unrelated.txt",
	}
	for _, n := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	out := captureStdout(t, func() { printDirContents(dir) })

	for _, want := range []string{
		"gosplit-backup-20260907T020000Z",
		"gosplit-pre-restore-20260907T030000Z",
		backup.MarkerName,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("listing omits %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "unrelated.txt") {
		t.Errorf("listing includes a non-archive file:\n%s", out)
	}
}

// TestPrintDirContentsEmpty asserts an empty directory says so, rather than
// printing nothing and leaving the operator unsure the command worked.
func TestPrintDirContentsEmpty(t *testing.T) {
	out := captureStdout(t, func() { printDirContents(t.TempDir()) })
	if !strings.Contains(out, "empty") {
		t.Errorf("an empty directory produced %q", out)
	}
}

// TestProbeDirLeavesNothing asserts repeated probes clean up after themselves.
func TestProbeDirLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := probeDir(dir); err != nil {
			t.Fatalf("probeDir: %v", err)
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

// TestPrepareDataDirsCreatesEverything asserts boot creates every configured
// directory. Distroless has no mkdir, so any it skips is one the operator
// cannot create in place either.
func TestPrepareDataDirsCreatesEverything(t *testing.T) {
	cfg := pathsCfg(t)
	cfg.AutoRestoreDir = filepath.Join(t.TempDir(), "restore")

	prepareDataDirs(cfg)

	for _, dir := range []string{
		store.SQLiteDataDir(cfg),
		cfg.UploadDir,
		cfg.BackupDir,
		cfg.AutoRestoreDir,
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("prepareDataDirs did not create %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

// TestPrepareDataDirsSkipsUnsetAutoRestore asserts an unset AUTO_RESTORE_DIR
// creates nothing. Creating it would switch on a destructive feature the
// operator did not ask for.
func TestPrepareDataDirsSkipsUnsetAutoRestore(t *testing.T) {
	cfg := pathsCfg(t)
	cfg.AutoRestoreDir = ""

	prepareDataDirs(cfg)

	// The configured directories are created; nothing else is.
	root := filepath.Dir(cfg.BackupDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name()] = true
	}
	for _, want := range []string{"db", "uploads", "backups"} {
		if !got[want] {
			t.Errorf("prepareDataDirs did not create %s", want)
		}
	}
	if len(got) != 3 {
		t.Errorf("created %v, want exactly db/uploads/backups -- an unconfigured AUTO_RESTORE_DIR must not switch on a destructive feature", entries)
	}
}

// TestPrepareDataDirsToleratesUnwritable asserts a bad directory is a warning,
// not a panic or an abort. Taking a working instance offline because backups
// are misconfigured would be worse than the misconfiguration.
func TestPrepareDataDirsToleratesUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("mode bits do not restrict the owner here")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	cfg := pathsCfg(t)
	cfg.BackupDir = filepath.Join(locked, "backups")

	// Must return normally.
	prepareDataDirs(cfg)

	// And the directories it could create, it did.
	if _, err := os.Stat(cfg.UploadDir); err != nil {
		t.Errorf("a failure on one directory stopped the others: %v", err)
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}
