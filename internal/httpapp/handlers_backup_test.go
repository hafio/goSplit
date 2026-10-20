package httpapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/store"
)

// adminHarness returns a harness signed in as an admin. admin@example.com is
// in the harness's AdminEmails, so registering promotes it.
func adminHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")
	return h
}

// userHarness returns a harness signed in as an ordinary user.
func userHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Plain", "plain@example.com", "password123")
	return h
}

// backupRoutes are every route the feature adds, with the method each needs.
var backupRoutes = []struct {
	method string
	path   string
}{
	{http.MethodGet, "/admin/backup"},
	{http.MethodPost, "/admin/backup/generate"},
	{http.MethodGet, "/admin/backup/status/sometoken"},
	{http.MethodGet, "/admin/backup/download/gosplit-backup-20260904T080000Z.gsbak"},
	{http.MethodPost, "/admin/backup/restore/upload"},
	{http.MethodPost, "/admin/backup/restore/confirm"},
}

// TestBackupRoutesRejectAnonymous asserts none of the admin routes is reachable
// without a session.
//
// The assertion deliberately looks at the UN-followed response. RequireAdmin
// sends an anonymous caller a 303 to /login, so a redirect-following client
// would report the login page's 200 and the test would pass or fail for
// reasons that have nothing to do with the guard.
func TestBackupRoutesRejectAnonymous(t *testing.T) {
	h := newHarness(t)
	for _, rt := range backupRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			resp := h.rawRequestNoRedirect(rt.method, rt.path, nil, "")
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode/100 == 2 {
				t.Fatalf("anonymous caller got %d for %s %s", resp.StatusCode, rt.method, rt.path)
			}
			// A GET is redirected to the sign-in page; a POST without a
			// session also fails CSRF. Either is a refusal, but a redirect
			// must go somewhere that is not the route itself.
			if resp.StatusCode/100 == 3 {
				loc := resp.Header.Get("Location")
				if !strings.HasPrefix(loc, "/login") {
					t.Errorf("%s %s redirected to %q, want /login", rt.method, rt.path, loc)
				}
			}
		})
	}
}

// TestBackupRoutesRejectNonAdmin asserts an ordinary signed-in user is
// forbidden. This is the check that matters: an archive holds every user's
// password hash.
func TestBackupRoutesRejectNonAdmin(t *testing.T) {
	h := userHarness(t)
	csrf := h.csrf()
	for _, rt := range backupRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			resp := h.rawRequest(rt.method, rt.path, nil, csrf)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == http.StatusOK {
				t.Errorf("non-admin got 200 for %s %s", rt.method, rt.path)
			}
			if rt.method == http.MethodGet && resp.StatusCode != http.StatusForbidden {
				t.Errorf("non-admin GET %s = %d, want 403", rt.path, resp.StatusCode)
			}
		})
	}
}

// TestBackupPageRendersForAdmin asserts the page renders, which also proves
// admin_backup.html is registered in the pageFiles map -- a page missing from
// it is never parsed and fails at render time.
func TestBackupPageRendersForAdmin(t *testing.T) {
	h := adminHarness(t)
	resp := h.get("/admin/backup")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/backup = %d", resp.StatusCode)
	}
	page := body(t, resp)
	// The typed confirmation phrase appears only once an archive has been
	// uploaded and validated, so it is not expected here.
	for _, want := range []string{
		"/admin/backup/generate",
		"/admin/backup/restore/upload",
		`enctype="multipart/form-data"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not mention %q", want)
		}
	}
	// The restore warning has to be on the initial page, not deferred to the
	// confirmation step.
	if !strings.Contains(page, "REPLACES ALL CURRENT DATA") {
		t.Error("page does not warn that a restore replaces everything")
	}
}

// TestBackupDownloadLinkNotBoosted is the regression guard for the htmx
// interaction: a boosted anchor swallows Content-Disposition, so the browser
// swaps in the response body instead of saving a file. Same reason
// /profile/export carries the attribute.
func TestBackupDownloadLinkNotBoosted(t *testing.T) {
	h := adminHarness(t)

	// Put a real archive on disk so the list renders a download link.
	h.mustGenerateBackup(t)

	resp := h.get("/admin/backup")
	defer func() { _ = resp.Body.Close() }()
	body := body(t, resp)

	i := strings.Index(body, "/admin/backup/download/")
	if i < 0 {
		t.Fatal("no download link rendered")
	}
	// Look at the anchor containing the link.
	start := strings.LastIndex(body[:i], "<a ")
	end := strings.Index(body[i:], ">")
	if start < 0 || end < 0 {
		t.Fatal("could not isolate the download anchor")
	}
	anchor := body[start : i+end]
	if !strings.Contains(anchor, `hx-boost="false"`) {
		t.Errorf("download anchor is missing hx-boost=\"false\", so htmx will swallow the download:\n %s", anchor)
	}
}

// TestBackupDownloadServesArchive asserts a real archive downloads with the
// attachment header.
func TestBackupDownloadServesArchive(t *testing.T) {
	h := adminHarness(t)
	name := h.mustGenerateBackup(t)

	resp := h.get("/admin/backup/download/" + name)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download = %d", resp.StatusCode)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.HasPrefix(b, []byte("GSPLTBAK")) {
		t.Error("downloaded body is not an archive")
	}
}

// TestBackupDownloadRejectsHostileNames asserts download-by-name cannot be
// turned into arbitrary file disclosure. The SQLite database sits in the same
// volume as the archives.
func TestBackupDownloadRejectsHostileNames(t *testing.T) {
	h := adminHarness(t)
	hostile := []string{
		"..%2F..%2Fgosplit.db",
		"..%2Fuploads%2Fa.png",
		"gosplit.db",
		"http.db",
		"gosplit-backup-20260904T080000Z.gsbak.partial",
		"evil.sh",
		"gosplit-backup-nonsense.gsbak",
	}
	for _, name := range hostile {
		t.Run(name, func(t *testing.T) {
			resp := h.get("/admin/backup/download/" + name)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == http.StatusOK {
				t.Errorf("download of %q succeeded", name)
			}
		})
	}
}

// TestRestoreUploadRejectsNonArchive asserts a junk upload is refused, and
// that refusing it changes nothing.
func TestRestoreUploadRejectsNonArchive(t *testing.T) {
	h := adminHarness(t)
	before := h.countUsers(t)

	resp := h.postFile(t, "/admin/backup/restore/upload", "archive", "junk.gsbak",
		[]byte("this is not an archive"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK && !strings.Contains(body(t, resp), "cannot be restored") {
		t.Error("a junk archive was accepted without an error shown")
	}
	if got := h.countUsers(t); got != before {
		t.Errorf("a rejected upload changed the users table (%d -> %d)", before, got)
	}
}

// TestRestoreUploadDiscardsThePendingOne: a new upload drops the restore that
// was waiting, file and all, even when the new archive is rejected --
// validating wipes the one staging directory either way, so the old pending
// restore could no longer be applied, and left in place it would expire later
// and delete the newer upload's staged files.
func TestRestoreUploadDiscardsThePendingOne(t *testing.T) {
	h := adminHarness(t)
	if err := os.MkdirAll(h.cfg.BackupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	waiting := filepath.Join(h.cfg.BackupDir, "upload-waiting.gsbak.tmp")
	if err := os.WriteFile(waiting, []byte("an earlier upload"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := h.app.Jobs.StagePending(&backup.Validated{}, waiting, 1)
	if err != nil {
		t.Fatal(err)
	}

	resp := h.postFile(t, "/admin/backup/restore/upload", "archive", "junk.gsbak", []byte("not an archive"))
	_ = body(t, resp)
	if _, ok := h.app.Jobs.PeekPending(token); ok {
		t.Error("the earlier pending restore survived a new upload")
	}
	if _, err := os.Stat(waiting); !os.IsNotExist(err) {
		t.Errorf("the earlier upload's file is still on disk (stat err %v)", err)
	}
}

// TestRestoreUploadRequiresCSRF asserts the upload is CSRF-protected. It sits
// on its own middleware chain, so this is worth asserting explicitly rather
// than assuming it inherits the group's protection.
func TestRestoreUploadRequiresCSRF(t *testing.T) {
	h := adminHarness(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("archive", "x.gsbak")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("irrelevant"))
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/admin/backup/restore/upload", &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	// Deliberately no csrf_token field and no X-CSRF-Token header.
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		t.Error("an upload without a CSRF token was accepted")
	}
}

// TestRestoreConfirmRejectsWrongPhrase asserts the typed confirmation is
// enforced server-side and that nothing is touched when it does not match.
func TestRestoreConfirmRejectsWrongPhrase(t *testing.T) {
	h := adminHarness(t)
	before := h.countUsers(t)

	for _, phrase := range []string{"", "restore all data", "RESTORE", "yes", "RESTORE ALL DATA "} {
		resp := h.post("/admin/backup/restore/confirm", url.Values{
			"token":   {"no-such-token"},
			"confirm": {phrase},
		})
		_ = resp.Body.Close()
		if got := h.countUsers(t); got != before {
			t.Fatalf("phrase %q changed the users table", phrase)
		}
	}
}

// TestRestoreConfirmRejectsUnknownToken asserts an expired or forged token
// cannot trigger a restore even with the right phrase.
func TestRestoreConfirmRejectsUnknownToken(t *testing.T) {
	h := adminHarness(t)
	before := h.countUsers(t)

	resp := h.post("/admin/backup/restore/confirm", url.Values{
		"token":   {"forged"},
		"confirm": {"RESTORE ALL DATA"},
	})
	defer func() { _ = resp.Body.Close() }()

	if got := h.countUsers(t); got != before {
		t.Error("a forged token triggered a restore")
	}
}

// TestBackupGenerateAndDownloadRoundTrip drives the panel end to end: generate
// a backup, poll until it finishes, then download it.
func TestBackupGenerateAndDownloadRoundTrip(t *testing.T) {
	h := adminHarness(t)
	name := h.mustGenerateBackup(t)

	if !strings.HasPrefix(name, "gosplit-backup-") || !strings.HasSuffix(name, ".gsbak") {
		t.Errorf("generated archive has an unexpected name: %q", name)
	}
	path := filepath.Join(h.cfg.BackupDir, name)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated archive: %v", err)
	}
	if info.Size() == 0 {
		t.Error("generated archive is empty")
	}
	// 0600: the archive holds password hashes and session tokens. Windows has
	// no POSIX mode bits -- Go reports 0666 there whatever was requested -- so
	// this is asserted only where it means something, which includes the
	// Linux container the image ships as.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("archive mode is %o, want no group or world access", perm)
		}
	}
}

// TestBackupJobStatusUnknownToken asserts an unknown token is a 404 rather
// than a zero-value status that would read as still running.
func TestBackupJobStatusUnknownToken(t *testing.T) {
	h := adminHarness(t)
	resp := h.get("/admin/backup/status/nope")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status for an unknown token = %d, want 404", resp.StatusCode)
	}
}

// TestRestoreStatusUnknownToken asserts the token-gated route also 404s, and
// in particular does not fall back to letting anyone through.
func TestRestoreStatusUnknownToken(t *testing.T) {
	h := newHarness(t) // deliberately anonymous
	resp := h.get("/admin/backup/restore/status/nope")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("restore status for an unknown token = %d, want 404", resp.StatusCode)
	}
}

// TestRestoreRoutesRefuseBackupTokens: the session-less progress page and
// status poll answer only a restore job's token. A finished backup job's token
// -- it sits in the generate redirect's URL -- is unknown there, so it can
// neither read the job nor sign anyone in. A finished restore signs its admin
// back in only when the id still carries the admin's address (control).
func TestRestoreRoutesRefuseBackupTokens(t *testing.T) {
	h := adminHarness(t)
	var adminID int64
	if err := h.st.DB.QueryRow(`SELECT id FROM users WHERE email = 'admin@example.com'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	done := func(context.Context) (backup.JobStatus, error) { return backup.JobStatus{State: backup.JobDone}, nil }
	start := func(run func(func(context.Context) (backup.JobStatus, error)) (string, error)) string {
		t.Helper()
		token, err := run(done)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 200; i++ {
			if st, _ := h.app.Jobs.Status(token); st.State == backup.JobDone {
				return token
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("job %s never finished", token)
		return ""
	}
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(path string) (*http.Response, bool) {
		t.Helper()
		resp, err := anon.Get(h.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		for _, c := range resp.Cookies() {
			if c.Name == auth.SessionCookie && c.Value != "" {
				return resp, true
			}
		}
		return resp, false
	}
	before := countOf(t, h, `SELECT COUNT(*) FROM sessions`)

	backupTok := start(func(fn func(context.Context) (backup.JobStatus, error)) (string, error) {
		return h.app.Jobs.Start(adminID, fn)
	})
	for _, path := range []string{"/admin/backup/restore/status/" + backupTok, "/admin/backup/restoring/" + backupTok} {
		if resp, signedIn := get(path); resp.StatusCode != http.StatusNotFound || signedIn {
			t.Errorf("GET %s with a backup token = %d (signed in: %v), want 404", path, resp.StatusCode, signedIn)
		}
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM sessions`); n != before {
		t.Errorf("sessions %d -> %d after polling with a backup token", before, n)
	}

	// The restored data gave the acting admin's id to someone with another
	// address: nobody is signed in.
	otherTok := start(func(fn func(context.Context) (backup.JobStatus, error)) (string, error) {
		return h.app.Jobs.StartRestore(adminID, "someone-else@example.com", fn)
	})
	if resp, signedIn := get("/admin/backup/restore/status/" + otherTok); resp.StatusCode != http.StatusOK || signedIn {
		t.Errorf("status for a restore whose id changed hands = %d (signed in: %v), want 200 and no session", resp.StatusCode, signedIn)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM sessions`); n != before {
		t.Errorf("sessions %d -> %d after a restore whose id changed hands", before, n)
	}

	restoreTok := start(func(fn func(context.Context) (backup.JobStatus, error)) (string, error) {
		return h.app.Jobs.StartRestore(adminID, "ADMIN@example.com", fn) // the address match ignores case
	})
	if resp, _ := get("/admin/backup/restoring/" + restoreTok); resp.StatusCode != http.StatusOK {
		t.Errorf("progress page for a restore token = %d, want 200", resp.StatusCode)
	}
	if resp, signedIn := get("/admin/backup/restore/status/" + restoreTok); resp.StatusCode != http.StatusOK || !signedIn {
		t.Errorf("status for a finished restore = %d (signed in: %v), want 200 and a session", resp.StatusCode, signedIn)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM sessions`); n != before+1 {
		t.Errorf("sessions %d -> %d, want one more for the re-established admin", before, n)
	}
}

// TestRestoreUploadCapIsEnforcedFirst: an upload over RESTORE_MAX_UPLOAD_MB is
// refused with 413 before any of it is read, so nothing is staged -- even when
// the archive in it is valid.
func TestRestoreUploadCapIsEnforcedFirst(t *testing.T) {
	h := adminHarness(t)
	archive, err := os.ReadFile(filepath.Join(h.cfg.BackupDir, h.mustGenerateBackup(t)))
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.RestoreMaxUploadMB = 1 // read when the router is built
	srv := httptest.NewServer(h.app.Router())
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("csrf_token", h.csrf())
	_ = mw.WriteField("filler", strings.Repeat("x", 2<<20))
	fw, err := mw.CreateFormFile("archive", "a.gsbak")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(archive)
	_ = mw.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/backup/restore/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := h.client.Do(req) // the jar keys cookies on the host, so the admin stays signed in
	if err != nil {
		t.Fatal(err)
	}

	if b := body(t, resp); resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(b, "RESTORE_MAX_UPLOAD_MB") {
		t.Errorf("oversized upload = %d %q, want 413 naming RESTORE_MAX_UPLOAD_MB", resp.StatusCode, b)
	}
	if staged, _ := filepath.Glob(filepath.Join(h.cfg.BackupDir, "upload-*.gsbak.tmp")); len(staged) != 0 {
		t.Errorf("a refused upload was buffered: %v", staged)
	}
	if _, err := os.Stat(h.cfg.UploadDir + ".restore"); !os.IsNotExist(err) {
		t.Errorf("a refused upload was staged (stat err %v)", err)
	}
}

// TestRestoreUploadChecksTheCallerBeforeParsing: an anonymous upload is sent
// to sign in by RequireUser, which now runs before VerifyCSRF parses the form,
// so a stranger's body is never read. (VerifyCSRF used to run first and answer
// 403 after parsing.)
func TestRestoreUploadChecksTheCallerBeforeParsing(t *testing.T) {
	h := newHarness(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("archive", "a.gsbak")
	_, _ = fw.Write([]byte("irrelevant"))
	_ = mw.Close()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/admin/backup/restore/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := anon.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/login") {
		t.Errorf("anonymous upload = %d to %q, want 303 to /login", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// TestExtendDeadlinesOutlastsTheServerTimeout: a body arriving more slowly
// than the server's ReadTimeout is still read in full once extendDeadlines has
// run -- what a large archive upload over an ordinary link needs.
func TestExtendDeadlinesOutlastsTheServerTimeout(t *testing.T) {
	srv := httptest.NewUnstartedServer(extendDeadlines(10 * time.Second)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			n, err := io.Copy(io.Discard, r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, n)
		})))
	srv.Config.ReadTimeout = 100 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)

	pr, pw := io.Pipe()
	go func() {
		for i := 0; i < 6; i++ {
			time.Sleep(60 * time.Millisecond)
			_, _ = pw.Write(bytes.Repeat([]byte("x"), 1024))
		}
		_ = pw.Close()
	}()
	resp, err := http.Post(srv.URL, "application/octet-stream", pr)
	if err != nil {
		t.Fatal(err)
	}
	if b := body(t, resp); resp.StatusCode != http.StatusOK || b != "6144" {
		t.Errorf("slow upload = %d %q, want 200 and all 6144 bytes", resp.StatusCode, b)
	}
}

// TestMaintenanceGateRefusesWrites asserts a mutating request is refused with
// 503 while a restore applies, instead of queueing behind the transaction and
// dying on an unexplained timeout.
func TestMaintenanceGateRefusesWrites(t *testing.T) {
	h := adminHarness(t)
	h.app.restoreInProgress.Store(true)
	defer h.app.restoreInProgress.Store(false)

	resp := h.post("/expenses", url.Values{"name": {"x"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("POST during a restore = %d, want 503", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("503 response is missing Retry-After")
	}
}

// TestMaintenanceGateAllowsReadsAndPoll asserts the gate is narrow: reads keep
// working, and the restore poll must keep working because it is how the
// operator learns the restore finished.
func TestMaintenanceGateAllowsReadsAndPoll(t *testing.T) {
	h := adminHarness(t)
	h.app.restoreInProgress.Store(true)
	defer h.app.restoreInProgress.Store(false)

	resp := h.get("/admin/backup")
	code := resp.StatusCode
	_ = resp.Body.Close()
	if code == http.StatusServiceUnavailable {
		t.Error("a GET was refused during a restore")
	}

	poll := h.get("/admin/backup/restore/status/whatever")
	pollCode := poll.StatusCode
	_ = poll.Body.Close()
	if pollCode == http.StatusServiceUnavailable {
		t.Error("the restore status poll was refused during a restore")
	}
}

// TestHealthzIncludesVersion asserts the build version reaches the health
// endpoint, so it is possible to tell which build a container is serving.
func TestHealthzIncludesVersion(t *testing.T) {
	h := newHarness(t)
	resp := h.get("/healthz")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz = %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /healthz: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q", body["status"])
	}
	if body["version"] != "v0.0.0-test" {
		t.Errorf("version = %q, want v0.0.0-test", body["version"])
	}
}

// TestFooterShowsVersion asserts the version reaches the rendered layout.
func TestFooterShowsVersion(t *testing.T) {
	h := adminHarness(t)
	resp := h.get("/balances")
	defer func() { _ = resp.Body.Close() }()
	body := body(t, resp)
	if !strings.Contains(body, `class="ver">v0.0.0-test<`) {
		t.Error("the footer does not show the build version")
	}
}

// TestAdminPageLinksToBackup asserts the panel is reachable, not just routed.
func TestAdminPageLinksToBackup(t *testing.T) {
	h := adminHarness(t)
	resp := h.get("/admin")
	defer func() { _ = resp.Body.Close() }()
	if !strings.Contains(body(t, resp), `href="/admin/backup"`) {
		t.Error("the admin page does not link to the backup page")
	}
}

// TestRestoreOverHTTP drives the panel's restore end to end. The preview shows
// the archive's token, the phrase to type and a row total that matches the
// dumped tables, and changes nothing. Confirming replaces the data and the
// uploads, writes a safety dump the status names, removes the uploaded file and
// the staged uploads, lifts the maintenance flag and spends the token.
func TestRestoreOverHTTP(t *testing.T) {
	h := adminHarness(t)
	if err := os.MkdirAll(h.cfg.UploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.cfg.UploadDir, "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	name := h.mustGenerateBackup(t)
	rows := 0
	for _, tbl := range backup.InsertOrder() {
		rows += countOf(t, h, `SELECT COUNT(*) FROM "`+tbl.Name+`"`)
	}
	h.addCarol(t)
	stray := filepath.Join(h.cfg.UploadDir, "stray.png")
	if err := os.WriteFile(stray, []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}

	token, page := h.stageRestore(t, filepath.Join(h.cfg.BackupDir, name))
	for _, want := range []string{`value="` + token + `"`, "<code>" + restoreConfirmPhrase + "</code>", fmt.Sprintf(`class="l-sub">%d rows`, rows)} {
		if !strings.Contains(page, want) {
			t.Errorf("the preview lacks %q", want)
		}
	}
	if n := h.countUsers(t); n != 2 {
		t.Errorf("users = %d after the preview, want 2 (nothing changed yet)", n)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("the preview touched the uploads: %v", err)
	}

	status, resp := h.confirmRestore(t, token)
	if status["state"] != "done" {
		t.Fatalf("restore ended %v: %v", status["state"], status["error"])
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("status Cache-Control = %q, want no-store", cc)
	}
	if n := h.countUsers(t); n != 1 {
		t.Errorf("users = %d after the restore, want the archive's 1", n)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("the uploads were not replaced (stray.png stat err %v)", err)
	}
	dump, _ := status["file"].(string)
	if !strings.HasPrefix(dump, backup.SafetyDumpPrefix) {
		t.Errorf("status names %q, want the safety dump", dump)
	} else if _, err := os.Stat(filepath.Join(h.cfg.BackupDir, dump)); err != nil {
		t.Errorf("the named safety dump is missing: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(h.cfg.BackupDir, "upload-*.gsbak.tmp")); len(left) != 0 {
		t.Errorf("the uploaded archive was left behind: %v", left)
	}
	if _, err := os.Stat(h.cfg.UploadDir + ".restore"); !os.IsNotExist(err) {
		t.Errorf("the staged uploads were left behind (stat err %v)", err)
	}

	replay := h.post("/admin/backup/restore/confirm", url.Values{"token": {token}, "confirm": {restoreConfirmPhrase}})
	if replay.StatusCode != http.StatusBadRequest || !strings.Contains(body(t, replay), "no longer pending") {
		t.Errorf("replaying the token = %d, want 400 saying it is no longer pending", replay.StatusCode)
	}
	if n := h.countUsers(t); n != 1 {
		t.Errorf("users = %d after the replay, want 1", n)
	}
}

// TestFailedRestoreOverHTTPChangesNothing: a restore that fails inside its
// transaction reports the error, leaves every table as it was and lifts the
// maintenance flag, so the next write is served rather than refused with 503.
func TestFailedRestoreOverHTTPChangesNothing(t *testing.T) {
	h := adminHarness(t)
	name := h.mustGenerateBackup(t)
	h.addCarol(t)
	token, _ := h.stageRestore(t, filepath.Join(h.cfg.BackupDir, name))
	before := tableCounts(t, h)
	if _, err := h.st.DB.Exec(`CREATE TRIGGER fail_users BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}

	status, _ := h.confirmRestore(t, token)
	if status["state"] != "error" {
		t.Fatalf("a restore whose inserts fail ended %v", status["state"])
	}
	if after := tableCounts(t, h); after != before {
		t.Errorf("a failed restore changed the tables:\nbefore %s\nafter  %s", before, after)
	}
	next := h.post("/admin/backup/restore/confirm", url.Values{"token": {token}, "confirm": {restoreConfirmPhrase}})
	_ = body(t, next)
	if next.StatusCode != http.StatusBadRequest {
		t.Errorf("a POST after the failed restore = %d, want 400 (the flag lifted, the token spent)", next.StatusCode)
	}
}

// TestRestoreUploadExplainsTheCommonRefusals: an archive sealed with another
// SESSION_SECRET, and one from a build with other migrations, each get their
// own message, and neither is staged.
func TestRestoreUploadExplainsTheCommonRefusals(t *testing.T) {
	ctx := context.Background()
	h := adminHarness(t)
	other := *h.cfg
	other.SessionSecret = "another-session-secret-0123456789"
	foreign, _, err := backup.New(h.st, &other).DumpToDir(ctx, t.TempDir(), backup.ArchivePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.st.DB.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES ('9999_future.sql', '2026-01-01T00:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	future, _, err := backup.New(h.st, h.cfg).DumpToDir(ctx, t.TempDir(), backup.ArchivePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.st.DB.Exec(`DELETE FROM schema_migrations WHERE version = '9999_future.sql'`); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, path, want string }{
		{"another SESSION_SECRET", foreign, "This archive was created with a different SESSION_SECRET:"},
		{"other migrations", future, "This archive does not match this version of GoSplit:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			resp := h.postFile(t, "/admin/backup/restore/upload", "archive", filepath.Base(tc.path), raw)
			page := body(t, resp)
			if resp.StatusCode != http.StatusBadRequest || !strings.Contains(page, tc.want) {
				t.Errorf("upload = %d, want 400 with %q", resp.StatusCode, tc.want)
			}
			if resp.Request.URL.Query().Get("pending") != "" {
				t.Error("a refused archive was staged for confirmation")
			}
			if left, _ := filepath.Glob(filepath.Join(h.cfg.BackupDir, "upload-*.gsbak.tmp")); len(left) != 0 {
				t.Errorf("the refused upload was left behind: %v", left)
			}
			if _, err := os.Stat(h.cfg.UploadDir + ".restore"); !os.IsNotExist(err) {
				t.Errorf("a refused archive left staged uploads (stat err %v)", err)
			}
		})
	}
}

// --- harness helpers -------------------------------------------------------

// rawRequest issues a bare request, attaching a CSRF token when given.
func (h *harness) rawRequest(method, path string, body io.Reader, csrf string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(method, h.srv.URL+path, body)
	if err != nil {
		h.t.Fatalf("NewRequest: %v", err)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("do %s %s: %v", method, path, err)
	}
	return resp
}

// rawRequestNoRedirect issues a bare request WITHOUT following redirects, so a
// test can see the refusal itself rather than wherever it points.
func (h *harness) rawRequestNoRedirect(method, path string, body io.Reader, csrf string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(method, h.srv.URL+path, body)
	if err != nil {
		h.t.Fatalf("NewRequest: %v", err)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	client := &http.Client{
		Jar:           h.client.Jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		h.t.Fatalf("do %s %s: %v", method, path, err)
	}
	return resp
}

// postFile posts a multipart body with one file field.
func (h *harness) postFile(t *testing.T, path, field, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("csrf_token", h.csrf()); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

// countUsers reads the users table directly, for asserting that a refused
// operation changed nothing.
func (h *harness) countUsers(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := h.st.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

// mustGenerateBackup drives the generate flow and returns the archive name.
func (h *harness) mustGenerateBackup(t *testing.T) string {
	t.Helper()
	resp := h.post("/admin/backup/generate", url.Values{})
	_ = resp.Body.Close()

	// The redirect carries the job token; poll until the job finishes.
	loc := resp.Request.URL.Query().Get("job")
	if loc == "" {
		t.Fatal("generate did not return a job token")
	}
	for i := 0; i < 400; i++ {
		st := h.get("/admin/backup/status/" + loc)
		var body map[string]any
		dec := json.NewDecoder(st.Body)
		_ = dec.Decode(&body)
		_ = st.Body.Close()
		switch body["state"] {
		case "done":
			name, _ := body["file"].(string)
			if name == "" {
				t.Fatal("finished backup job reported no file")
			}
			return name
		case "error":
			t.Fatalf("backup job failed: %v", body["error"])
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backup job did not finish in time")
	return ""
}

// addCarol adds a user straight to the store, so a restore of an earlier
// backup has something visible to undo.
func (h *harness) addCarol(t *testing.T) {
	t.Helper()
	if _, err := h.st.CreateUser(context.Background(), &store.User{Name: "Carol", Email: "carol@example.com"}); err != nil {
		t.Fatal(err)
	}
}

// tableCounts is every registry table's row count, as one comparable string.
func tableCounts(t *testing.T, h *harness) string {
	t.Helper()
	var sb strings.Builder
	for _, tbl := range backup.InsertOrder() {
		fmt.Fprintf(&sb, "%s=%d ", tbl.Name, countOf(t, h, `SELECT COUNT(*) FROM "`+tbl.Name+`"`))
	}
	return sb.String()
}

// stageRestore uploads the archive at path and returns the token its preview
// is addressed by, with the preview page.
func (h *harness) stageRestore(t *testing.T, path string) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	resp := h.postFile(t, "/admin/backup/restore/upload", "archive", filepath.Base(path), raw)
	page := body(t, resp)
	token := resp.Request.URL.Query().Get("pending")
	if token == "" {
		t.Fatalf("the upload was not staged (status %d):\n%s", resp.StatusCode, page)
	}
	return token, page
}

// confirmRestore confirms a staged restore, then polls its job until it ends.
// It returns the final status body and the response that carried it.
func (h *harness) confirmRestore(t *testing.T, token string) (map[string]any, *http.Response) {
	t.Helper()
	resp := h.post("/admin/backup/restore/confirm", url.Values{"token": {token}, "confirm": {restoreConfirmPhrase}})
	_ = resp.Body.Close()
	job, ok := strings.CutPrefix(resp.Request.URL.Path, "/admin/backup/restoring/")
	if !ok {
		t.Fatalf("confirm landed on %s, not the progress page", resp.Request.URL.Path)
	}
	for i := 0; i < 400; i++ {
		st := h.get("/admin/backup/restore/status/" + job)
		var out map[string]any
		_ = json.NewDecoder(st.Body).Decode(&out)
		_ = st.Body.Close()
		if out["state"] == "done" || out["state"] == "error" {
			return out, st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the restore did not finish in time")
	return nil, nil
}
