package backup

import (
	"fmt"
	"os"

	"github.com/hafio/gosplit/internal/config"
)

// The upload swap is two renames and cannot join the SQL transaction, so a
// crash in between leaves a recognizable state on disk. RecoverIncompleteSwap
// runs at the very start of every boot, before the database is opened, and
// puts things back.
//
// The fixed directory suffixes (see stagingDirFor and oldDirFor) exist for
// exactly this: a timestamped name would be unrecognizable to a later process.

// RecoverIncompleteSwap repairs an upload swap interrupted by a crash.
//
// Three states are possible:
//
//   - Only the staging directory exists. The crash happened before the
//     database committed, so the database is untouched and the staged uploads
//     are irrelevant. Delete them.
//   - Only the .old directory exists, and the live directory is present. The
//     swap finished but the cleanup did not. Delete the leftover.
//   - The .old directory exists and the live directory does not. The crash
//     landed between the two renames: uploads and database may now disagree.
//     Move .old back so the instance has an upload directory, then fail loudly
//     -- an operator needs to know the restore may have to be re-run.
func RecoverIncompleteSwap(cfg *config.Config) error {
	if cfg.UploadDir == "" {
		return nil
	}
	live := cfg.UploadDir
	staging := stagingDirFor(live)
	old := oldDirFor(live)

	stagingExists := dirExists(staging)
	oldExists := dirExists(old)
	liveExists := dirExists(live)

	if stagingExists && !oldExists {
		// Pre-commit crash, or a validated restore that was never confirmed.
		if err := os.RemoveAll(staging); err != nil {
			return fmt.Errorf("backup: remove stale upload staging directory %s: %w", staging, err)
		}
	}

	if !oldExists {
		return nil
	}

	if liveExists {
		// The swap completed; only the cleanup was interrupted.
		if err := os.RemoveAll(old); err != nil {
			return fmt.Errorf("backup: remove leftover upload directory %s: %w", old, err)
		}
		return nil
	}

	// Interrupted between the two renames.
	if err := os.Rename(old, live); err != nil {
		return fmt.Errorf("backup: a restore was interrupted while swapping the upload directory, and %s could not be moved back to %s: %w -- restore the uploads by hand before starting again", old, live, err)
	}
	_ = os.RemoveAll(staging)
	return fmt.Errorf("backup: a restore was interrupted while swapping the upload directory. The previous uploads have been moved back into %s, but the database may hold the restored data, so uploads and database may disagree. Re-run the restore, or restore the pre-restore safety dump from %s", live, cfg.BackupDir)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
