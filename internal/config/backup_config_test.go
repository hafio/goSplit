package config

import (
	"strings"
	"testing"
)

// backupEnv sets the environment to a valid baseline so each test below can
// change exactly the one variable it is about.
func backupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SESSION_SECRET", validSecret)
	t.Setenv("BACKUP_DIR", "/data/backups")
	t.Setenv("BACKUP_CRON", "")
	t.Setenv("BACKUP_RETENTION_COUNT", "")
	t.Setenv("AUTO_RESTORE_DIR", "")
	t.Setenv("RESTORE_MAX_UPLOAD_MB", "")
	t.Setenv("RESTORE_MAX_ARCHIVE_MB", "")
	t.Setenv("RESTORE_MAX_ENTRIES", "")
	t.Setenv("RESTORE_MAX_TABLE_FILE_MB", "")
	t.Setenv("RESTORE_MAX_JSONL_LINE_MB", "")
}

// TestBackupDefaults pins the defaults, including that both the schedule and
// the startup auto-restore are off unless asked for. Neither should ever turn
// itself on: one writes files, the other replaces the whole database.
func TestBackupDefaults(t *testing.T) {
	backupEnv(t)
	t.Setenv("BACKUP_DIR", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.BackupCron != "" {
		t.Errorf("BackupCron = %q, want empty (scheduled backups off by default)", c.BackupCron)
	}
	if c.AutoRestoreDir != "" {
		t.Errorf("AutoRestoreDir = %q, want empty (auto-restore off by default)", c.AutoRestoreDir)
	}
	if c.BackupDir != "./data/backups" {
		t.Errorf("BackupDir = %q", c.BackupDir)
	}
	if c.BackupRetentionCount != 7 {
		t.Errorf("BackupRetentionCount = %d, want 7", c.BackupRetentionCount)
	}
	if c.RestoreMaxUploadMB != 500 {
		t.Errorf("RestoreMaxUploadMB = %d, want 500", c.RestoreMaxUploadMB)
	}
	if c.RestoreMaxArchiveBytes != 2048<<20 {
		t.Errorf("RestoreMaxArchiveBytes = %d, want %d", c.RestoreMaxArchiveBytes, int64(2048)<<20)
	}
	if c.RestoreMaxEntries != 200000 {
		t.Errorf("RestoreMaxEntries = %d", c.RestoreMaxEntries)
	}
	// AppVersion is set by main from the link-time stamp, never from the
	// environment.
	if c.AppVersion != "" {
		t.Errorf("AppVersion = %q, want empty from Load", c.AppVersion)
	}
}

// TestBackupCronAccepted covers valid 5-field rules, using the same parser the
// scheduler uses.
func TestBackupCronAccepted(t *testing.T) {
	for _, rule := range []string{"0 4 * * *", "* * * * *", "*/15 * * * *", "30 2 1 * *", "0 0 * * 0"} {
		backupEnv(t)
		t.Setenv("BACKUP_CRON", rule)
		if _, err := Load(); err != nil {
			t.Errorf("BACKUP_CRON %q was rejected: %v", rule, err)
		}
	}
}

// TestBackupCronInvalidFailsLoud asserts a bad rule stops the boot. A
// scheduled backup that silently never fires is the worst outcome here --
// nobody notices until they need the backup.
func TestBackupCronInvalidFailsLoud(t *testing.T) {
	for _, rule := range []string{
		"not a cron rule",
		"0 4 * *",     // four fields
		"0 4 * * * *", // six fields
		"99 4 * * *",  // minute out of range
		"@daily",      // descriptor, not accepted by this parser
	} {
		backupEnv(t)
		t.Setenv("BACKUP_CRON", rule)
		_, err := Load()
		if err == nil {
			t.Errorf("BACKUP_CRON %q was accepted", rule)
			continue
		}
		if !strings.Contains(err.Error(), "BACKUP_CRON") {
			t.Errorf("error for %q should name BACKUP_CRON: %v", rule, err)
		}
	}
}

// TestBackupCronWithoutDirFailsLoud asserts a schedule with nowhere to write
// is a misconfiguration rather than a silent no-op.
func TestBackupCronWithoutDirFailsLoud(t *testing.T) {
	backupEnv(t)
	t.Setenv("BACKUP_CRON", "0 4 * * *")
	t.Setenv("BACKUP_DIR", " ")

	// A single space is not empty, so it passes; the real case is an operator
	// clearing BACKUP_DIR while leaving the cron rule set. getEnv treats an
	// empty value as unset and substitutes the default, so force the field
	// directly to cover the validation itself.
	c := &Config{BackupCron: "0 4 * * *", BackupDir: "", BackupRetentionCount: 7,
		RestoreMaxUploadMB: 1, RestoreMaxArchiveBytes: 1, RestoreMaxEntries: 1,
		RestoreMaxTableFileBytes: 1, RestoreMaxJSONLLineBytes: 1}
	err := c.validateBackup()
	if err == nil {
		t.Fatal("a cron rule with no BACKUP_DIR was accepted")
	}
	if !strings.Contains(err.Error(), "BACKUP_DIR") {
		t.Errorf("error should name BACKUP_DIR: %v", err)
	}
}

// TestBackupDirEqualAutoRestoreDirFailsLoud is the sharpest of these. If the
// scheduled backup wrote into the watched directory, the next boot would
// restore the instance's own most recent backup -- an automatic, silent
// rollback of everything since.
func TestBackupDirEqualAutoRestoreDirFailsLoud(t *testing.T) {
	for _, pair := range [][2]string{
		{"/data/backups", "/data/backups"},
		{"/data/backups", "/data/backups/"},
		{"/data/backups", "/data/./backups"},
	} {
		backupEnv(t)
		t.Setenv("BACKUP_DIR", pair[0])
		t.Setenv("AUTO_RESTORE_DIR", pair[1])

		_, err := Load()
		if err == nil {
			t.Errorf("BACKUP_DIR %q and AUTO_RESTORE_DIR %q were both accepted", pair[0], pair[1])
			continue
		}
		if !strings.Contains(err.Error(), "AUTO_RESTORE_DIR") {
			t.Errorf("error should name AUTO_RESTORE_DIR: %v", err)
		}
	}
}

// TestBackupDirDifferentFromAutoRestoreDirAccepted is the positive case, so
// the check above cannot be tightened into rejecting a valid setup.
func TestBackupDirDifferentFromAutoRestoreDirAccepted(t *testing.T) {
	backupEnv(t)
	t.Setenv("BACKUP_DIR", "/data/backups")
	t.Setenv("AUTO_RESTORE_DIR", "/data/restore")

	c, err := Load()
	if err != nil {
		t.Fatalf("two distinct directories were rejected: %v", err)
	}
	if c.AutoRestoreDir != "/data/restore" {
		t.Errorf("AutoRestoreDir = %q", c.AutoRestoreDir)
	}
}

// TestBackupRetentionValidation asserts zero is rejected -- it reads as
// "keep none", which would delete every archive right after writing it --
// while a negative value is the documented way to keep everything.
func TestBackupRetentionValidation(t *testing.T) {
	backupEnv(t)
	t.Setenv("BACKUP_RETENTION_COUNT", "0")
	if _, err := Load(); err == nil {
		t.Error("BACKUP_RETENTION_COUNT=0 was accepted")
	}

	backupEnv(t)
	t.Setenv("BACKUP_RETENTION_COUNT", "-1")
	c, err := Load()
	if err != nil {
		t.Fatalf("a negative retention count should mean keep everything: %v", err)
	}
	if c.BackupRetentionCount != -1 {
		t.Errorf("BackupRetentionCount = %d, want -1", c.BackupRetentionCount)
	}
}

// TestRestoreLimitsMustBePositive asserts a zero cap is refused. A zero would
// reject every archive, turning a typo into an unrestorable instance.
func TestRestoreLimitsMustBePositive(t *testing.T) {
	for _, v := range []string{"RESTORE_MAX_UPLOAD_MB", "RESTORE_MAX_ARCHIVE_MB",
		"RESTORE_MAX_ENTRIES", "RESTORE_MAX_TABLE_FILE_MB", "RESTORE_MAX_JSONL_LINE_MB"} {
		backupEnv(t)
		t.Setenv(v, "-1")
		_, err := Load()
		if err == nil {
			t.Errorf("%s=-1 was accepted", v)
			continue
		}
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error should name %s: %v", v, err)
		}
	}
}

// TestBackupScheduleParserMatchesRecurrences asserts the exported parser
// accepts the same 5-field shape expense recurrences use, so an operator does
// not have to learn two cron dialects.
func TestBackupScheduleParserMatchesRecurrences(t *testing.T) {
	sched, err := BackupSchedule("0 4 * * *")
	if err != nil {
		t.Fatalf("BackupSchedule: %v", err)
	}
	if sched == nil {
		t.Fatal("BackupSchedule returned a nil schedule")
	}
	if _, err := BackupSchedule("@every 1h"); err == nil {
		t.Error("a descriptor form was accepted; the 5-field parser should reject it")
	}
}

// TestSameDir covers the path comparison behind the directory-collision check.
func TestSameDir(t *testing.T) {
	same := [][2]string{
		{"/a/b", "/a/b"},
		{"/a/b/", "/a/b"},
		{"/a/./b", "/a/b"},
		{"/a/c/../b", "/a/b"},
	}
	for _, p := range same {
		if !sameDir(p[0], p[1]) {
			t.Errorf("sameDir(%q, %q) = false, want true", p[0], p[1])
		}
	}
	diff := [][2]string{
		{"/a/b", "/a/c"},
		{"/a/b", "/a/b/c"},
		{"", "/a"},
	}
	for _, p := range diff {
		if sameDir(p[0], p[1]) {
			t.Errorf("sameDir(%q, %q) = true, want false", p[0], p[1])
		}
	}
}
