package httpapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hafio/gosplit/internal/backup"
)

// Admin backup and restore. Handlers stay thin, but two things here are not
// the usual shape and are deliberate:
//
//   - Generate and restore run as background jobs. The server's WriteTimeout
//     is 60s and handlers use a 10s context, neither of which survives a real
//     backup, so the request returns a job token and the page polls.
//   - The restore status poll is gated by that token rather than by a session.
//     A restore replaces the users table, and sessions cascade-delete with
//     users, so the caller's own session row is gone by the time the restore
//     finishes. A session-gated poll could never observe its own success.

// restoreConfirmPhrase is what an admin must type to confirm. Deliberately not
// localized: it has to be unambiguous whichever locale the panel is rendered
// in, and it is compared server-side.
const restoreConfirmPhrase = "RESTORE ALL DATA"

// archiveNameRe bounds a download filename. Without it, download-by-name is
// arbitrary file disclosure -- the SQLite database sits in the same volume.
var archiveNameRe = regexp.MustCompile(`^gosplit-(backup|pre-restore)-[0-9]{8}T[0-9]{6}Z\.gsbak$`)

// backupTimeout bounds a whole dump or restore. Generous: these run detached
// from any request.
const backupTimeout = 2 * time.Hour

// handleBackupPage renders the backup and restore page.
func (s *Server) handleBackupPage(w http.ResponseWriter, r *http.Request) {
	s.renderBackupPage(w, r, http.StatusOK, "", "")
}

// renderBackupPage lists the archives on disk plus any in-flight job.
func (s *Server) renderBackupPage(w http.ResponseWriter, r *http.Request, status int, errMsg, flash string) {
	archives, err := s.listArchives()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Archives":      archives,
		"JobToken":      r.URL.Query().Get("job"),
		"BackupDir":     s.Cfg.BackupDir,
		"ScheduleCron":  s.Cfg.BackupCron,
		"Retention":     s.Cfg.BackupRetentionCount,
		"AutoRestoreOn": s.Cfg.AutoRestoreDir != "",
		"MaxUploadMB":   s.Cfg.RestoreMaxUploadMB,
		"ConfirmPhrase": restoreConfirmPhrase,
		// The archive carries password hashes and session tokens, so warn when
		// it would be downloaded over plain HTTP.
		"Insecure": !s.Cfg.SecureCookies,
	}
	// A validated-but-unconfirmed upload is addressed by token in the query,
	// so a reload keeps the preview rather than losing the staged archive.
	if r.URL.Query().Get("badphrase") != "" && errMsg == "" {
		errMsg = s.tr(r, "err.restore_confirm")
	}
	if token := r.URL.Query().Get("pending"); token != "" {
		if man, ok := s.Jobs.PeekPending(token); ok {
			var rows int64
			for _, t := range man.Tables {
				rows += t.RowCount
			}
			data["Pending"] = man
			data["PendingToken"] = token
			data["PendingRows"] = rows
		} else if errMsg == "" {
			errMsg = s.tr(r, "err.restore_expired")
		}
	}

	vd := s.vdPage(w, r, "title.backup", data)
	vd.Error = errMsg
	if flash != "" {
		vd.Flash = flash
	}
	s.Renderer.Render(w, status, "admin_backup", vd)
}

// archiveInfo is one archive on disk, for the listing.
type archiveInfo struct {
	Name     string
	Size     int64
	Modified string
	IsSafety bool
}

// listArchives returns the archives in BackupDir, newest first.
func (s *Server) listArchives() ([]archiveInfo, error) {
	dir := s.Cfg.BackupDir
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []archiveInfo
	for _, e := range entries {
		if e.IsDir() || !archiveNameRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, archiveInfo{
			Name:     e.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format("2006-01-02 15:04"),
			IsSafety: len(e.Name()) > len(backup.SafetyDumpPrefix) && e.Name()[:len(backup.SafetyDumpPrefix)] == backup.SafetyDumpPrefix,
		})
	}
	// Names carry a fixed-width UTC timestamp, so reverse lexicographic order
	// is newest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// handleBackupGenerate starts a backup job and redirects to the page.
func (s *Server) handleBackupGenerate(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	runner := backup.New(s.Store, s.Cfg)
	dir := s.Cfg.BackupDir

	token, err := s.Jobs.Start(u.ID, func(ctx context.Context) (backup.JobStatus, error) {
		ctx, cancel := context.WithTimeout(ctx, backupTimeout)
		defer cancel()
		path, man, err := runner.DumpToDir(ctx, dir, backup.ArchivePrefix)
		if err != nil {
			return backup.JobStatus{}, err
		}
		return backup.JobStatus{State: backup.JobDone, ResultPath: path, Manifest: &man}, nil
	})
	if err != nil {
		s.renderBackupPage(w, r, http.StatusInternalServerError, s.tr(r, "err.backup_generate"), "")
		return
	}
	http.Redirect(w, r, "/admin/backup?job="+token, http.StatusSeeOther)
}

// handleBackupJobStatus reports a backup job's state. Admin-gated, unlike the
// restore poll: a backup does not destroy the caller's session.
func (s *Server) handleBackupJobStatus(w http.ResponseWriter, r *http.Request) {
	st, ok := s.Jobs.Status(chi.URLParam(r, "token"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJobJSON(w, st)
}

// handleBackupDownload streams an archive.
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	// The name comes from a URL, so it is validated against a fixed pattern
	// and then confirmed to resolve inside BackupDir. Either check alone would
	// be weaker than both.
	if !archiveNameRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.Cfg.BackupDir, name)
	if filepath.Dir(path) != filepath.Clean(s.Cfg.BackupDir) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
	// A large archive can outrun the server's write timeout, so extend this
	// response's deadline rather than loosening it for every route.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(backupTimeout))
	_, _ = io.Copy(w, f)
}

// handleRestoreUpload accepts an archive, validates it, and shows a preview.
// Nothing is changed here: validation stages the uploads beside the live
// directory and touches no table.
func (s *Server) handleRestoreUpload(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)

	// The body was already bounded by maxUploadBytes middleware, which must
	// run before CSRF verification -- see the route registration.
	file, _, err := r.FormFile("archive")
	if err != nil {
		s.renderBackupPage(w, r, http.StatusBadRequest, s.tr(r, "err.restore_no_file"), "")
		return
	}
	defer func() { _ = file.Close() }()

	// Buffer to BackupDir, not os.TempDir: the runtime image has a read-only
	// root filesystem and only the data volume is writable. Routed through the
	// shared check so a wrong-ownership bind mount produces the same
	// actionable message here as it does on the command line.
	if err := backup.EnsureWritableDir(s.Cfg.BackupDir, "BACKUP_DIR"); err != nil {
		s.renderBackupPage(w, r, http.StatusInternalServerError, err.Error(), "")
		return
	}
	tmp, err := os.CreateTemp(s.Cfg.BackupDir, "upload-*.gsbak.tmp")
	if err != nil {
		s.renderBackupPage(w, r, http.StatusInternalServerError, err.Error(), "")
		return
	}
	tmpPath := tmp.Name()
	_, copyErr := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		msg := s.tr(r, "err.restore_upload")
		if copyErr != nil {
			// A body over the cap surfaces here as a read error.
			msg = fmt.Sprintf("%s (%s)", msg, copyErr)
		}
		s.renderBackupPage(w, r, http.StatusBadRequest, msg, "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	runner := backup.New(s.Store, s.Cfg)
	v, err := runner.Validate(ctx, tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		s.renderBackupPage(w, r, http.StatusBadRequest, s.restoreErrMessage(r, err), "")
		return
	}

	token, err := s.Jobs.StagePending(v, tmpPath, u.ID)
	if err != nil {
		v.Discard()
		_ = os.Remove(tmpPath)
		s.renderBackupPage(w, r, http.StatusInternalServerError, err.Error(), "")
		return
	}
	http.Redirect(w, r, "/admin/backup?pending="+token, http.StatusSeeOther)
}

// handleRestoreConfirm applies a validated archive after the typed phrase
// matches. This is the destructive step.
func (s *Server) handleRestoreConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if r.FormValue("confirm") != restoreConfirmPhrase {
		// The pending archive is deliberately NOT consumed, so the admin can
		// retype the phrase without re-uploading.
		http.Redirect(w, r, "/admin/backup?pending="+token+"&badphrase=1", http.StatusSeeOther)
		return
	}
	v, path, actingID, ok := s.Jobs.TakePending(token)
	if !ok {
		s.renderBackupPage(w, r, http.StatusBadRequest, s.tr(r, "err.restore_expired"), "")
		return
	}

	runner := backup.New(s.Store, s.Cfg)
	jobToken, err := s.Jobs.Start(actingID, func(ctx context.Context) (backup.JobStatus, error) {
		ctx, cancel := context.WithTimeout(ctx, backupTimeout)
		defer cancel()

		// Refuse concurrent mutations for the duration, and clear the flag on
		// every exit path.
		s.restoreInProgress.Store(true)
		defer s.restoreInProgress.Store(false)
		defer func() { _ = os.Remove(path) }()
		defer v.Discard()

		rep, err := runner.Apply(ctx, v, backup.ApplyOptions{Confirmed: true})
		if err != nil {
			return backup.JobStatus{}, err
		}
		return backup.JobStatus{State: backup.JobDone, Manifest: &rep.Manifest, ResultPath: rep.SafetyDumpPath}, nil
	})
	if err != nil {
		v.Discard()
		_ = os.Remove(path)
		s.renderBackupPage(w, r, http.StatusInternalServerError, err.Error(), "")
		return
	}
	http.Redirect(w, r, "/admin/backup/restoring/"+jobToken, http.StatusSeeOther)
}

// handleRestoreProgress renders the waiting page for a running restore. It is
// outside the admin group for the same reason the poll is: by the time the
// restore finishes, the caller's session row no longer exists.
func (s *Server) handleRestoreProgress(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if _, ok := s.Jobs.Status(token); !ok {
		http.NotFound(w, r)
		return
	}
	vd := s.vd(r, "title.backup", map[string]any{"JobToken": token, "Restoring": true})
	s.Renderer.Render(w, http.StatusOK, "admin_backup", vd)
}

// handleRestoreStatus reports a restore job's state, gated by the job token
// rather than a session.
//
// On completion it re-establishes the acting admin's session if that user
// still exists in the restored data -- looked up by id against the live table,
// never taken from the archive. A background goroutine has no ResponseWriter,
// so this live request is the only place a cookie can be set.
func (s *Server) handleRestoreStatus(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	st, ok := s.Jobs.Status(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if st.State == backup.JobDone && st.ActingUserID != 0 {
		s.reauthenticateAfterRestore(w, r, st.ActingUserID)
	}
	writeJobJSON(w, st)
}

// reauthenticateAfterRestore signs the acting admin back in when their user
// survived the restore, and clears the stale cookie when it did not.
func (s *Server) reauthenticateAfterRestore(w http.ResponseWriter, r *http.Request, userID int64) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()

	if u, err := s.Store.GetUser(ctx, userID); err == nil && u != nil {
		if s.currentUser(r) == nil {
			if err := s.Auth.SetSession(w, r, u.ID); err != nil {
				slog.Warn("backup: could not re-establish the admin session after a restore", "err", err)
			}
		}
		return
	}
	// The acting admin is not in the restored data; drop the dead cookie.
	s.Auth.ClearSession(w, r)
}

// writeJobJSON renders a job's state for the page's poll.
func writeJobJSON(w http.ResponseWriter, st backup.JobStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	body := map[string]any{"state": string(st.State)}
	if st.Err != "" {
		body["error"] = st.Err
	}
	if st.ResultPath != "" {
		body["file"] = filepath.Base(st.ResultPath)
	}
	if st.Manifest != nil {
		body["tables"] = len(st.Manifest.Tables)
		body["uploads"] = st.Manifest.UploadFileCount
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("backup: could not encode job status", "err", err)
	}
}

// restoreErrMessage turns a validation failure into something an operator can
// act on, keeping the two most common causes distinct.
func (s *Server) restoreErrMessage(r *http.Request, err error) string {
	switch {
	case errors.Is(err, backup.ErrWrongSessionSecret):
		return s.tr(r, "err.restore_wrong_secret") + " " + err.Error()
	case errors.Is(err, backup.ErrSchemaMismatch):
		return s.tr(r, "err.restore_schema") + " " + err.Error()
	default:
		return s.tr(r, "err.restore_invalid") + " " + err.Error()
	}
}

// maxUploadBytes bounds a request body before any middleware parses it.
//
// This has to run BEFORE VerifyCSRF. That middleware falls back to
// r.FormValue("csrf_token"), which calls ParseMultipartForm and would
// otherwise buffer an unbounded upload -- spilling to a filesystem the
// container may not even have -- before the handler's own cap could apply.
func maxUploadBytes(mb int) func(http.Handler) http.Handler {
	limit := int64(mb) << 20
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
