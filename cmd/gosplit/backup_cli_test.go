package main

import (
	"os"
	"strings"
	"testing"
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
