package backup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArchivePrefix is the name prefix DumpToDir uses for a scheduled or manual
// backup, and SafetyDumpPrefix the one it uses for a pre-restore dump. They are
// distinct so retention can prune the two independently, and so the startup
// auto-restore never mistakes a safety dump for an operator's input.
const (
	ArchivePrefix     = "gosplit-backup"
	SafetyDumpPrefix  = "gosplit-pre-restore"
	partialSuffix     = ".partial"
	stalePartialAfter = 24 * time.Hour
)

// PruneOldArchives keeps the newest `keep` archives matching prefix in dir and
// removes the rest, returning how many it removed. A keep of zero or less
// means keep everything.
//
// Archive names carry a UTC timestamp in a fixed-width format, so a
// lexicographic sort is chronological -- no need to stat every file.
func PruneOldArchives(dir, prefix string, keep int) (int, error) {
	if dir == "" || keep <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("backup: read %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, prefix+"-") && strings.HasSuffix(n, ArchiveExt) {
			names = append(names, n)
		}
	}
	if len(names) <= keep {
		return 0, nil
	}
	sort.Strings(names)

	removed := 0
	var firstErr error
	// Oldest first, keeping the tail.
	for _, n := range names[:len(names)-keep] {
		p := filepath.Join(dir, n)
		if err := os.Remove(p); err != nil {
			// One stubborn file should not abort the sweep, but it is not
			// swallowed either.
			slog.Warn("backup: could not prune archive", "path", p, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	if firstErr != nil {
		return removed, fmt.Errorf("backup: pruned %d archive(s) but at least one could not be removed: %w", removed, firstErr)
	}
	return removed, nil
}

// SweepStalePartials removes leftover .partial files older than a day. One is
// left behind only when a dump is killed mid-write; nothing reads them, so
// without a sweep they would accumulate silently.
func SweepStalePartials(dir string, now time.Time) int {
	if dir == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), partialSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < stalePartialAfter {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}
