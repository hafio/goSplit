package backup

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureWritableDir creates dir if it is missing and proves this process can
// write inside it, before any caller does real work.
//
// The check exists because the failure it catches is the most common
// deployment mistake for this image and, unguarded, produces an unhelpful
// error. A host directory bind-mounted for backups is typically root-owned,
// while the container runs as an unprivileged UID -- so os.MkdirAll succeeds
// (the directory is already there) and the failure only surfaces later, as a
// bare "permission denied" on a file path, with nothing to say who the process
// is or what to change.
//
// setting names the knob that produced dir (BACKUP_DIR, -o, AUTO_RESTORE_DIR)
// so the operator knows which one to look at.
func EnsureWritableDir(dir, setting string) error {
	if dir == "" {
		return fmt.Errorf("backup: no directory configured (%s is empty)", setting)
	}

	// A missing directory whose parent is not writable is a different problem
	// with a different fix, so the two are reported separately: here the chown
	// target is the parent, below it is the directory itself.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("backup: cannot create %s (from %s): %w -- this process runs as uid %d, and %s must be writable by it. On a bind mount, fix it on the host with: chown %d:%d %s",
				dir, setting, mkErr,
				os.Getuid(), filepath.Dir(dir),
				os.Getuid(), os.Getgid(), filepath.Dir(dir))
		}
	}

	if err := probeWritable(dir); err != nil {
		return fmt.Errorf("backup: %s (from %s) is not writable: %w -- this process runs as uid %d gid %d. On a bind mount the host directory is usually root-owned; fix it on the host with: chown -R %d:%d <host path>",
			dir, setting, err,
			os.Getuid(), os.Getgid(),
			os.Getuid(), os.Getgid())
	}
	return nil
}

// probeWritable checks that dir accepts a new file, by creating and removing
// one. Stat and mode bits are not enough on their own: a read-only mount, a
// full filesystem and an ACL all present differently, and a write is the only
// thing that answers the question a caller actually has.
func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".gosplit-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}
