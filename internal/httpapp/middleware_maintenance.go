package httpapp

import (
	"net/http"
	"strings"
)

// While a restore is applying, the database is being emptied and refilled
// inside one transaction. On SQLite that transaction holds the pool's only
// connection (store.connect pins it to one), so every other request would
// queue behind it and then fail on the handler's 10s context timeout with
// nothing to explain why.
//
// This gate turns that into an honest answer: mutating requests get a 503 with
// Retry-After while the restore runs. Reads are left alone -- on SQLite they
// will still block on the connection, but a reader that waits and then
// succeeds is better than one refused for no visible reason.
//
// It only covers a restore triggered in this process, from the admin panel. A
// CLI restore runs as a separate process with no access to the flag; on
// Postgres the LOCK TABLE in ApplyRestore covers both cases at the engine
// level, and on SQLite the CLI path is documented as a maintenance window.

// retryAfterSeconds is what a refused client is told to wait. Short enough to
// retry promptly, long enough not to hammer a busy restore.
const retryAfterSeconds = "30"

// maintenanceGate refuses mutating requests while a restore is in flight.
func (s *Server) maintenanceGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.restoreInProgress.Load() || isSafeMethod(r.Method) || isRestorePoll(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", retryAfterSeconds)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("GoSplit is restoring a backup, so changes are paused. Retry in a moment.\n"))
	})
}

// isSafeMethod reports whether a method only reads.
func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// isRestorePoll reports whether a path is the restore status poll, which must
// keep working during a restore -- it is how the operator learns the restore
// finished.
func isRestorePoll(path string) bool {
	return strings.HasPrefix(path, "/admin/backup/restore/status/")
}
