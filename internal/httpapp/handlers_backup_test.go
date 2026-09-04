package httpapp

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
