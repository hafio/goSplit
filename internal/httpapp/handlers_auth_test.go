package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestSafeNextRefusesOffSiteTargets: a sign-in "next" that a browser would
// follow off the site -- an absolute URL, a protocol-relative one, a backslash
// (read as a slash) or a control character (stripped) -- falls back to "/". A
// local path is kept as it is.
func TestSafeNextRefusesOffSiteTargets(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/groups", "/groups"},
		{"/groups/1?all=1#feed", "/groups/1?all=1#feed"},
		{"", "/"},
		{"https://evil.example", "/"},
		{"//evil.example", "/"},
		{`/\evil.example`, "/"},
		{"/\t/evil.example", "/"},
		{"/\n/evil.example", "/"},
	} {
		if got := safeNext(tc.in); got != tc.want {
			t.Errorf("safeNext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLogoutClearsSiteData: signing out tells the browser to drop the service
// worker's offline copies of the account's pages, so the next person on a
// shared device cannot read them offline.
func TestLogoutClearsSiteData(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)
	resp := h.post("/logout", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Clear-Site-Data") != `"cache", "storage"` {
		t.Errorf("logout = %d with Clear-Site-Data %q, want 303 clearing cache and storage",
			resp.StatusCode, resp.Header.Get("Clear-Site-Data"))
	}
}

// TestAdminPromotionWaitsForVerification: registering an ADMIN_EMAILS address
// does not make it an admin -- anyone could register it first. Signing in with
// a link sent to the address does.
func TestAdminPromotionWaitsForVerification(t *testing.T) {
	h := newHarness(t)
	_ = body(t, h.get("/login")) // seed the CSRF cookie
	_ = body(t, h.post("/register", url.Values{"name": {"Root"}, "email": {"admin@example.com"}, "password": {"password123"}}))
	if resp := h.get("/admin"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("/admin right after registering an admin address = %d, want 403", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// The owner opens a sign-in link that reached the address.
	if err := h.st.CreateVerificationToken(context.Background(), "admin@example.com", "tok-verify", "magic", time.Hour); err != nil {
		t.Fatal(err)
	}
	_ = body(t, h.get("/auth/magic?token=tok-verify"))
	if resp := h.get("/admin"); resp.StatusCode != http.StatusOK {
		t.Errorf("/admin after a link sign-in = %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}
