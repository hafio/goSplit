package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// TestWantsBackup covers the subcommand dispatch. It is separate from
// wantsVersion so that function and its existing table stay untouched.
func TestWantsBackup(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
		wantOK  bool
	}{
		{"no args", []string{"gosplit"}, "", false},
		{"backup", []string{"gosplit", "backup"}, "backup", true},
		{"backup with flags", []string{"gosplit", "backup", "-o", "/data"}, "backup", true},
		{"restore", []string{"gosplit", "restore", "--file", "x.gsbak"}, "restore", true},
		{"inspect", []string{"gosplit", "inspect", "x.gsbak"}, "inspect", true},
		{"paths", []string{"gosplit", "paths"}, "paths", true},
		// A bare invocation must still start the server, so anything the
		// dispatcher does not recognize falls through.
		{"unknown subcommand", []string{"gosplit", "serve"}, "", false},
		{"version is not a backup command", []string{"gosplit", "version"}, "", false},
		{"flag is not a subcommand", []string{"gosplit", "--help"}, "", false},
		{"case sensitive", []string{"gosplit", "Backup"}, "", false},
		{"subcommand must be first", []string{"gosplit", "serve", "backup"}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ok := wantsBackup(tc.args)
			if ok != tc.wantOK || cmd != tc.wantCmd {
				t.Errorf("wantsBackup(%v) = (%q, %v), want (%q, %v)", tc.args, cmd, ok, tc.wantCmd, tc.wantOK)
			}
		})
	}
}

// TestWantsBackupAndVersionAreDisjoint asserts the two dispatchers cannot both
// claim the same argument, which would make the order of the checks in main()
// a silent behaviour decision.
func TestWantsBackupAndVersionAreDisjoint(t *testing.T) {
	for _, arg := range []string{"version", "-version", "--version", "backup", "restore", "inspect", "paths"} {
		args := []string{"gosplit", arg}
		_, isBackup := wantsBackup(args)
		isVersion := wantsVersion(args)
		if isBackup && isVersion {
			t.Errorf("%q is claimed by both dispatchers", arg)
		}
	}
}

// TestRunBackupCommandRejectsUnknown asserts the dispatcher does not silently
// accept a command wantsBackup never returns.
func TestRunBackupCommandRejectsUnknown(t *testing.T) {
	err := runBackupCommand("nonsense", nil)
	if err == nil {
		t.Fatal("an unknown subcommand was accepted")
	}
	if !strings.Contains(err.Error(), "nonsense") {
		t.Errorf("error should name the subcommand: %v", err)
	}
}

// TestRunRestoreCLIRequiresFile asserts --file is mandatory, and that the
// check happens before any configuration or database work -- so the failure
// does not depend on the environment.
func TestRunRestoreCLIRequiresFile(t *testing.T) {
	err := runRestoreCLI(nil)
	if err == nil {
		t.Fatal("restore without --file was accepted")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Errorf("error should name the missing flag: %v", err)
	}
}

// TestRunInspectCLIRequiresExactlyOnePath covers the argument guard. inspect
// deliberately needs no SESSION_SECRET and no database, so these paths are
// reachable without any environment set up.
func TestRunInspectCLIRequiresExactlyOnePath(t *testing.T) {
	for _, args := range [][]string{nil, {"a.gsbak", "b.gsbak"}} {
		if err := runInspectCLI(args); err == nil {
			t.Errorf("inspect %v was accepted", args)
		}
	}
}

// TestRunInspectCLIReportsMissingFile asserts inspect fails on a path that is
// not an archive, rather than printing an empty header.
func TestRunInspectCLIReportsMissingFile(t *testing.T) {
	err := runInspectCLI([]string{t.TempDir() + "/does-not-exist.gsbak"})
	if err == nil {
		t.Fatal("inspecting a missing file succeeded")
	}
}

// TestInspectRejectsNonArchive asserts a file that is not an archive is
// refused by the magic check, before any key derivation.
func TestInspectRejectsNonArchive(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/not-an-archive.gsbak"
	if err := os.WriteFile(path, []byte("this is plainly not a GoSplit archive"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := runInspectCLI([]string{path})
	if err == nil {
		t.Fatal("a non-archive was accepted")
	}
	if !strings.Contains(err.Error(), "not a GoSplit archive") {
		t.Errorf("error should say what is wrong: %v", err)
	}
}

// TestHumanBytes covers the size formatting shown in the CLI summary.
func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1 << 20, "1.0 MiB"},
		{1 << 30, "1.0 GiB"},
	}
	for _, tc := range tests {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestOrDash keeps an empty field from rendering as nothing at all.
func TestOrDash(t *testing.T) {
	for in, want := range map[string]string{"": "-", "   ": "-", "v1.2.3": "v1.2.3"} {
		if got := orDash(in); got != want {
			t.Errorf("orDash(%q) = %q, want %q", in, got, want)
		}
	}
}

// cliEnv points the commands at a fresh SQLite instance under one temp root,
// in the state a server's first boot leaves it: migrated, with one user and one
// upload. It returns the config the commands will load.
func cliEnv(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DATABASE_URL", "file:"+filepath.Join(dir, "gosplit.db"))
	t.Setenv("UPLOAD_DIR", filepath.Join(dir, "uploads"))
	t.Setenv("BACKUP_DIR", filepath.Join(dir, "backups"))
	t.Setenv("AUTO_RESTORE_DIR", "")
	t.Setenv("BACKUP_CRON", "")
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	st, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = st.Close()
	withStore(t, cfg, func(st *store.Store) { addUser(t, st, "alice@example.com") })
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.UploadDir, "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// withStore opens the instance's database without migrating it, runs fn and
// closes it again, so the command under test holds the only open handle.
func withStore(t *testing.T, cfg *config.Config, fn func(*store.Store)) {
	t.Helper()
	st, err := store.OpenNoMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()
	fn(st)
}

func addUser(t *testing.T, st *store.Store, email string) {
	t.Helper()
	if _, err := st.CreateUser(context.Background(), &store.User{Name: email, Email: email}); err != nil {
		t.Fatalf("create %s: %v", email, err)
	}
}

func countUsers(t *testing.T, cfg *config.Config) int {
	t.Helper()
	var n int
	withStore(t, cfg, func(st *store.Store) {
		if err := st.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
			t.Fatalf("count users: %v", err)
		}
	})
	return n
}

// archivesIn lists the archives in dir whose names start with prefix.
func archivesIn(t *testing.T, dir, prefix string) []string {
	t.Helper()
	got, err := filepath.Glob(filepath.Join(dir, prefix+"*"+backup.ArchiveExt))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// cliBackup runs a plain `gosplit backup` and returns the archive it wrote.
func cliBackup(t *testing.T, cfg *config.Config) string {
	t.Helper()
	var err error
	captureStderr(t, func() { err = runBackupCLI(nil) })
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	got := archivesIn(t, cfg.BackupDir, backup.ArchivePrefix)
	if len(got) != 1 {
		t.Fatalf("archives after one backup = %v", got)
	}
	return got[0]
}

// diverge makes the live instance differ from a backup just taken: Bob signs
// up and a stray upload appears.
func diverge(t *testing.T, cfg *config.Config) {
	t.Helper()
	withStore(t, cfg, func(st *store.Store) { addUser(t, st, "bob@example.com") })
	if err := os.WriteFile(filepath.Join(cfg.UploadDir, "stray.png"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestBackupCLIWritesOneArchive: `gosplit backup -o DIR` writes exactly one
// archive into DIR and nothing into BACKUP_DIR; without -o it goes to
// BACKUP_DIR. The report names the file.
func TestBackupCLIWritesOneArchive(t *testing.T) {
	cfg := cliEnv(t)
	other := t.TempDir()

	var err error
	out := captureStderr(t, func() { err = runBackupCLI([]string{"-o", other}) })
	if err != nil {
		t.Fatalf("backup -o: %v", err)
	}
	if got := archivesIn(t, other, ""); len(got) != 1 || !strings.Contains(out, "wrote "+got[0]) {
		t.Errorf("backup -o wrote %v, reported:\n%s", got, out)
	}
	if got := archivesIn(t, cfg.BackupDir, ""); len(got) != 0 {
		t.Errorf("backup -o also wrote into BACKUP_DIR: %v", got)
	}

	cliBackup(t, cfg)
}

// TestBackupCLIRefusesAnUnmigratedDatabase: a database this build would still
// migrate is neither backed up nor migrated. The backup typically runs right
// before an upgrade and must not change the schema as a side effect.
func TestBackupCLIRefusesAnUnmigratedDatabase(t *testing.T) {
	ctx := context.Background()
	cfg := cliEnv(t)
	var last string
	withStore(t, cfg, func(st *store.Store) {
		applied, err := st.AppliedMigrations(ctx)
		if err != nil || len(applied) == 0 {
			t.Fatalf("applied migrations = %v, %v", applied, err)
		}
		last = applied[len(applied)-1]
		if _, err := st.DB.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = ?`, last); err != nil {
			t.Fatal(err)
		}
	})

	var err error
	captureStderr(t, func() { err = runBackupCLI(nil) })
	if err == nil || !strings.Contains(err.Error(), last) || !strings.Contains(err.Error(), "Start the server once") {
		t.Fatalf("backup of an unmigrated database = %v, want a refusal naming %s", err, last)
	}
	if got := archivesIn(t, cfg.BackupDir, ""); len(got) != 0 {
		t.Errorf("a refused backup wrote %v", got)
	}
	withStore(t, cfg, func(st *store.Store) {
		if pending, err := st.PendingMigrations(ctx); err != nil || len(pending) != 1 || pending[0] != last {
			t.Errorf("pending after the refused backup = %v, %v; want %s still pending", pending, err, last)
		}
	})
}

// TestRestoreCLIWithoutForceChangesNothing: without --force, restore only
// validates. The data and uploads stay, no safety dump is written and no
// staged uploads are left beside UPLOAD_DIR.
func TestRestoreCLIWithoutForceChangesNothing(t *testing.T) {
	cfg := cliEnv(t)
	archive := cliBackup(t, cfg)
	diverge(t, cfg)

	var err error
	out := captureStderr(t, func() { err = runRestoreCLI([]string{"--file", archive}) })
	if err != nil || !strings.Contains(out, "Nothing has been changed") {
		t.Fatalf("restore without --force = %v, reported:\n%s", err, out)
	}
	if n := countUsers(t, cfg); n != 2 {
		t.Errorf("users = %d after a validate-only restore, want 2", n)
	}
	if _, err := os.Stat(filepath.Join(cfg.UploadDir, "stray.png")); err != nil {
		t.Errorf("a validate-only restore touched the uploads: %v", err)
	}
	if got := archivesIn(t, cfg.BackupDir, backup.SafetyDumpPrefix); len(got) != 0 {
		t.Errorf("a validate-only restore wrote a safety dump: %v", got)
	}
	if _, err := os.Stat(cfg.UploadDir + ".restore"); !os.IsNotExist(err) {
		t.Errorf("staged uploads were left behind (stat err %v)", err)
	}
}

// TestRestoreCLIForceRestores: --force replaces the data and the uploads with
// the archive's, and first writes a safety dump of what was there -- one that
// restores, and holds the pre-restore rows. --no-pre-dump skips only that dump.
func TestRestoreCLIForceRestores(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
		dumps int
	}{
		{"with the safety dump", nil, 1},
		{"--no-pre-dump", []string{"--no-pre-dump"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := cliEnv(t)
			archive := cliBackup(t, cfg)
			diverge(t, cfg)

			var err error
			captureStderr(t, func() { err = runRestoreCLI(append([]string{"--file", archive, "--force"}, tc.extra...)) })
			if err != nil {
				t.Fatalf("restore --force: %v", err)
			}
			if n := countUsers(t, cfg); n != 1 {
				t.Errorf("users after the restore = %d, want the archive's 1", n)
			}
			if _, err := os.Stat(filepath.Join(cfg.UploadDir, "stray.png")); !os.IsNotExist(err) {
				t.Errorf("the uploads were not replaced (stray.png stat err %v)", err)
			}
			if _, err := os.Stat(filepath.Join(cfg.UploadDir, "a.png")); err != nil {
				t.Errorf("the archive's upload is missing: %v", err)
			}
			dumps := archivesIn(t, cfg.BackupDir, backup.SafetyDumpPrefix)
			if len(dumps) != tc.dumps {
				t.Fatalf("safety dumps = %v, want %d", dumps, tc.dumps)
			}
			for _, d := range dumps {
				withStore(t, cfg, func(st *store.Store) {
					v, err := backup.New(st, cfg).Validate(ctx, d)
					if err != nil {
						t.Fatalf("the safety dump does not restore: %v", err)
					}
					defer v.Discard()
					if n, _ := v.Manifest.RowCount("users"); n != 2 {
						t.Errorf("the safety dump holds %d users, want the pre-restore 2", n)
					}
				})
			}
		})
	}
}
