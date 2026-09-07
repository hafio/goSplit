package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// Startup auto-restore. When AUTO_RESTORE_DIR holds an archive and the marker
// file is absent, the instance restores that archive and then writes the
// marker. The check is bare presence, by decision: no checksum, and the
// archive is left where it is.
//
// EVERY failure here is fatal and aborts the boot, schema-version mismatch
// included. That is a deliberate choice with a real cost: because the marker
// is written only on success, an archive that cannot be restored will fail the
// boot again on every restart, and under `restart: unless-stopped` that is an
// outage with no automatic way out. The compensating measure is that every
// fatal message below names the one-step remediation, so the fix is obvious
// from the log alone.

const (
	// MarkerName is the file whose presence means "a restore has already run
	// here". Its contents are informational only; only its existence is
	// checked.
	MarkerName = ".gosplit-restored"

	// autoRestoreLockID is the leader lock that stops several replicas sharing
	// one Postgres database from all restoring at once.
	autoRestoreLockID = "auto-restore"

	// autoRestoreLockTTL is renewed while the restore runs, so a slow restore
	// cannot let the lock lapse and a second replica start its own.
	autoRestoreLockTTL = 2 * time.Minute
)

// CheckAutoRestore performs the startup auto-restore if one is configured and
// pending. It must be called after migrations have run -- the live schema has
// to match the binary before an archive can be validated against it -- and
// before the HTTP listener and scheduler start, so nothing else can write
// while the restore is in flight.
func CheckAutoRestore(ctx context.Context, st *store.Store, cfg *config.Config) error {
	dir := cfg.AutoRestoreDir
	if dir == "" {
		return nil
	}

	marker := filepath.Join(dir, MarkerName)
	if _, err := os.Stat(marker); err == nil {
		slog.Info("auto-restore: already done, skipping", "marker", marker)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("auto-restore: cannot read the marker file %s: %w -- fix the permissions on %s, or unset AUTO_RESTORE_DIR, then restart", marker, err, dir)
	}

	archive, err := findAutoRestoreArchive(dir)
	if err != nil {
		return err
	}
	if archive == "" {
		slog.Info("auto-restore: no archive found, nothing to do", "dir", dir)
		return nil
	}

	// Prove the marker can be written BEFORE restoring. Without this probe a
	// read-only directory would mean a successful restore is never recorded,
	// and the instance would wipe and re-restore on every single restart.
	if err := EnsureWritableDir(dir, "AUTO_RESTORE_DIR"); err != nil {
		return fmt.Errorf("auto-restore: %s holds %s but the marker file cannot be written there: %w -- a restore would repeat on every restart, so it was not attempted. Make the directory writable, or unset AUTO_RESTORE_DIR, then restart", dir, filepath.Base(archive), err)
	}

	leader, release, err := acquireWithRenewal(ctx, st, autoRestoreLockID, autoRestoreLockTTL)
	if err != nil {
		return fmt.Errorf("auto-restore: cannot take the %q lock: %w", autoRestoreLockID, err)
	}
	if !leader {
		// Another replica is restoring. Booting normally is correct here: it
		// will not serve stale data for long, and two concurrent whole-database
		// wipes would be far worse.
		slog.Warn("auto-restore: another instance holds the restore lock, skipping", "archive", archive)
		return nil
	}
	defer release()

	slog.Info("auto-restore: starting", "archive", archive)
	r := New(st, cfg)

	v, err := r.Validate(ctx, archive)
	if err != nil {
		return fmt.Errorf("auto-restore: %s cannot be restored: %w -- the database was NOT changed. Remove %s from %s, or unset AUTO_RESTORE_DIR, then restart", filepath.Base(archive), err, filepath.Base(archive), dir)
	}
	defer v.Discard()

	rep, err := r.Apply(ctx, v, ApplyOptions{Confirmed: true, SafetyDumpDir: dir})
	if err != nil {
		return fmt.Errorf("auto-restore: restoring %s failed: %w -- the transaction was rolled back, so the database is unchanged. Remove %s from %s, or unset AUTO_RESTORE_DIR, then restart", filepath.Base(archive), err, filepath.Base(archive), dir)
	}

	if err := writeMarker(marker, archive, rep); err != nil {
		// The restore succeeded but is unrecorded, so the next restart would
		// wipe the freshly restored data and do it all again. Refusing to
		// serve is the safer end of that trade.
		return fmt.Errorf("auto-restore: %s was restored successfully, but the marker file %s could not be written: %w -- restarting now would repeat the restore. Create that file by hand, or unset AUTO_RESTORE_DIR, then restart", filepath.Base(archive), marker, err)
	}

	slog.Info("auto-restore: complete",
		"archive", archive,
		"uploads", rep.UploadsRestored,
		"safety_dump", rep.SafetyDumpPath,
		"marker", marker)
	return nil
}

// findAutoRestoreArchive returns the single archive to restore from dir.
//
// More than one is an error rather than a guess: picking the newest would be a
// silent decision about which of an operator's files replaces the whole
// database.
func findAutoRestoreArchive(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("auto-restore: AUTO_RESTORE_DIR does not exist", "dir", dir)
			return "", nil
		}
		return "", fmt.Errorf("auto-restore: cannot read %s: %w -- fix the path or unset AUTO_RESTORE_DIR, then restart", dir, err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// .partial is a dump still being written; the safety-dump prefix marks
		// archives this feature wrote itself.
		if !strings.HasSuffix(name, ArchiveExt) || strings.HasPrefix(name, SafetyDumpPrefix) {
			continue
		}
		found = append(found, name)
	}
	sort.Strings(found)
	if len(found) > 1 {
		return "", fmt.Errorf("auto-restore: %s holds %d archives (%s) and it is ambiguous which should replace the database -- leave exactly one, then restart", dir, len(found), strings.Join(found, ", "))
	}
	if len(found) == 0 {
		return "", nil
	}
	return filepath.Join(dir, found[0]), nil
}

// writeMarker records that a restore has run. Only the file's existence is
// ever checked; the contents are for whoever reads it later.
func writeMarker(path, archive string, rep Report) error {
	body := fmt.Sprintf(
		"restored %s\nat %s\narchive created %s by gosplit %s\nsafety dump %s\n",
		filepath.Base(archive),
		time.Now().UTC().Format(isoFormat),
		rep.Manifest.CreatedAt,
		orDashStr(rep.Manifest.GosplitVersion),
		orDashStr(rep.SafetyDumpPath),
	)
	return os.WriteFile(path, []byte(body), 0o644)
}

func orDashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// acquireWithRenewal takes a leader lock and keeps renewing it in the
// background until the returned release is called.
//
// The renewal matters: a single acquire with a fixed TTL that a slow restore
// outlives would let a second replica take the lock and start its own wipe
// while the first is still running. This mirrors how the scheduler keeps its
// "cleanup" lock fresh by re-acquiring on every tick.
func acquireWithRenewal(ctx context.Context, st *store.Store, id string, ttl time.Duration) (bool, func(), error) {
	holder, _ := os.Hostname()
	holder = fmt.Sprintf("%s-%d", holder, os.Getpid())

	leader, err := st.AcquireLock(ctx, id, holder, ttl)
	if err != nil {
		return false, nil, err
	}
	if !leader {
		return false, func() {}, nil
	}

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(ttl / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if ok, err := st.AcquireLock(ctx, id, holder, ttl); err != nil {
					slog.Warn("lock renewal failed", "lock", id, "err", err)
				} else if !ok {
					slog.Warn("lock was taken by another holder", "lock", id)
				}
			}
		}
	}()
	return true, func() { close(done) }, nil
}
