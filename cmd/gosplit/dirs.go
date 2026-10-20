package main

import (
	"log/slog"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// prepareBoot readies the data volume before anything opens the database:
// first it repairs an upload swap a crash interrupted, then it creates the
// directories this instance writes to.
//
// The order is the whole point. RecoverIncompleteSwap tells "interrupted
// between the two renames" from "finished" by whether UPLOAD_DIR exists, so
// creating a missing UPLOAD_DIR first would make an interrupted swap look
// finished -- and recovery would then delete the previous uploads it exists to
// put back.
func prepareBoot(cfg *config.Config) error {
	if err := backup.RecoverIncompleteSwap(cfg); err != nil {
		return err
	}
	prepareDataDirs(cfg)
	return nil
}

// prepareDataDirs creates every directory this instance writes to, at boot,
// and reports on any it cannot.
//
// Two reasons it happens here rather than lazily at each first use. The runtime
// image is distroless -- no shell, no mkdir -- so a directory the app does not
// create is one an operator cannot create either, except through a throwaway
// container mounted against the same volume. And a wrong-ownership bind mount
// is far better discovered in the boot log than at 04:00 when the first
// scheduled backup fails.
//
// A failure here is logged, not fatal. An unwritable BACKUP_DIR breaks backups,
// which matters, but taking a working instance offline over it would be worse;
// the backup itself then fails loudly with the same actionable message.
// AUTO_RESTORE_DIR keeps its own separate fatal path in CheckAutoRestore, which
// runs later and does abort the boot -- there, an unrecordable restore really
// cannot be allowed to proceed.
func prepareDataDirs(cfg *config.Config) {
	type target struct {
		setting string
		path    string
	}

	targets := []target{
		{"UPLOAD_DIR", cfg.UploadDir},
		{"BACKUP_DIR", cfg.BackupDir},
	}
	// SQLite only: store.connect creates this too, but doing it here means a
	// permission problem is reported with the uid and the remedy rather than
	// surfacing later as an opaque ping failure.
	if dir := store.SQLiteDataDir(cfg); dir != "" {
		targets = append([]target{{"DATABASE_URL", dir}}, targets...)
	}
	if cfg.AutoRestoreDir != "" {
		targets = append(targets, target{"AUTO_RESTORE_DIR", cfg.AutoRestoreDir})
	}

	// Every target has a path: config.Load never leaves UPLOAD_DIR or
	// BACKUP_DIR empty, and the optional ones are only added when set.
	for _, t := range targets {
		if err := backup.EnsureWritableDir(t.path, t.setting); err != nil {
			slog.Warn("startup: a configured directory is not usable",
				"setting", t.setting, "path", t.path, "err", err)
			continue
		}
		slog.Debug("startup: directory ready", "setting", t.setting, "path", t.path)
	}
}
