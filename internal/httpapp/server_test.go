package httpapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRequestBodiesAreCappedBeforeCSRF: every body on the shared chain is
// capped at UPLOAD_MAX_FILE_SIZE_MB plus 1 MiB before VerifyCSRF parses it, so
// an anonymous caller cannot make the server read or spool a large body.
func TestRequestBodiesAreCappedBeforeCSRF(t *testing.T) {
	h := newHarness(t)
	h.cfg.UploadMaxFileSizeMB = 1 // a 2 MiB cap; read when the router is built
	srv := httptest.NewServer(h.app.Router())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/login", "application/x-www-form-urlencoded",
		strings.NewReader("email=a%40b.c&password=x&pad="+strings.Repeat("x", 3<<20)))
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	if b := body(t, resp); resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(b, "UPLOAD_MAX_FILE_SIZE_MB") {
		t.Errorf("oversized anonymous POST = %d %q, want 413 naming UPLOAD_MAX_FILE_SIZE_MB", resp.StatusCode, b)
	}
}

// TestUploadsServesFilesButNoListing: avatars stay public by URL, but a
// directory -- the root or any other -- is a 404, so nobody can enumerate the
// avatar names.
func TestUploadsServesFilesButNoListing(t *testing.T) {
	h := newHarness(t)
	name := "3f1c2a9e-1b7d-4c55-9a0e-8d2f6b1e4c3a.png"
	if err := os.MkdirAll(filepath.Join(h.cfg.UploadDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.cfg.UploadDir, name), []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(h.srv.URL + "/uploads/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if b := body(t, resp); resp.StatusCode != http.StatusOK || b != "fake-png" ||
		resp.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Errorf("GET the avatar = %d %q (Cache-Control %q), want 200 with its bytes",
			resp.StatusCode, b, resp.Header.Get("Cache-Control"))
	}
	for _, dir := range []string{"/uploads/", "/uploads/sub/"} {
		resp, err := http.Get(h.srv.URL + dir)
		if err != nil {
			t.Fatal(err)
		}
		if b := body(t, resp); resp.StatusCode != http.StatusNotFound || strings.Contains(b, name) {
			t.Errorf("GET %s = %d, want 404 without a listing:\n%s", dir, resp.StatusCode, b)
		}
	}
}

// TestHealthzReportsALostDatabase: /healthz answers 503 once the database is
// unreachable -- the signal a container orchestrator restarts on.
func TestHealthzReportsALostDatabase(t *testing.T) {
	h := newHarness(t)
	_ = h.st.Close()
	resp := h.get("/healthz")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/healthz with the database closed = %d, want 503", resp.StatusCode)
	}
}
