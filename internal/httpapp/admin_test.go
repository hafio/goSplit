package httpapp

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/auth"
)

// TestAdminUserManagement covers the admin page editing a user's details and
// generating a magic-link URL shown inline in that user's card.
func TestAdminUserManagement(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123") // AdminEmails -> ADMIN
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	page := body(t, h.get("/admin"))
	if !strings.Contains(page, "adm-exp") || !strings.Contains(page, "bob@example.com") {
		t.Fatalf("admin page missing collapsible user rows")
	}
	if !strings.Contains(page, `class="tiles"`) {
		t.Errorf("admin page missing stat tiles")
	}

	// Edit bob (id 2): rename + promote. POST follows the redirect to /admin,
	// which renders the confirmation carried in the one-shot flash cookie.
	saved := body(t, h.post("/admin/users/2", url.Values{
		"name": {"Bobby"}, "email": {"bob@example.com"},
		"role": {"ADMIN"}, "currency": {"USD"}, "language": {"en"},
	}))
	if !strings.Contains(saved, "User updated") {
		t.Errorf("admin edit missing confirmation flash")
	}
	if after := body(t, h.get("/admin")); !strings.Contains(after, "Bobby") {
		t.Errorf("admin edit did not persist name")
	}

	// Magic link renders inline in bob's card.
	resp := h.post("/admin/users/2/magic", url.Values{})
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(b, "/auth/magic?token=") || !strings.Contains(b, "magic-inline") {
		t.Fatalf("magic link not shown inline: status %d", resp.StatusCode)
	}
}

// TestAdminCreateUser covers creation and duplicate-email rejection.
func TestAdminCreateUser(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")

	if resp := h.post("/admin/users/create", url.Values{
		"email": {"carol@example.com"}, "name": {"Carol"},
		"role": {"USER"}, "currency": {"USD"}, "language": {"en"},
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	if !strings.Contains(body(t, h.get("/admin")), "carol@example.com") {
		t.Errorf("created user not listed")
	}

	// Duplicate email → 400 + error banner (no redirect).
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/admin/users/create", url.Values{"email": {"carol@example.com"}})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(b, "already exists") {
		t.Fatalf("duplicate email not rejected: status %d", resp.StatusCode)
	}
	h.client.CheckRedirect = nil
}

// TestAdminSetPasswordRevokesTargetSession confirms setting a password logs the
// target out everywhere and lets them log in with the new one.
func TestAdminSetPasswordRevokesTargetSession(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123") // id 1
	_ = body(t, h.post("/admin/users/create", url.Values{
		"email": {"victim@example.com"}, "password": {"password12"}, "role": {"USER"},
	})) // id 2

	// A second client logs in as the victim.
	jar, _ := cookiejar.New(nil)
	victim := &http.Client{Jar: jar}
	su, _ := url.Parse(h.srv.URL)
	_, _ = victim.Get(h.srv.URL + "/login") // seed CSRF cookie
	var csrf string
	for _, c := range jar.Cookies(su) {
		if c.Name == auth.CSRFCookie {
			csrf = c.Value
		}
	}
	if _, err := victim.PostForm(h.srv.URL+"/login", url.Values{
		"email": {"victim@example.com"}, "password": {"password12"}, "csrf_token": {csrf},
	}); err != nil {
		t.Fatal(err)
	}
	if r, _ := victim.Get(h.srv.URL + "/balances"); r.StatusCode != http.StatusOK {
		t.Fatalf("victim not logged in: %d", r.StatusCode)
	}

	// Admin sets a new password → victim's session is revoked.
	_ = body(t, h.post("/admin/users/2/password", url.Values{"password": {"brandnew99"}}))

	victim.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if r, _ := victim.Get(h.srv.URL + "/balances"); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("victim session not revoked (status %d)", r.StatusCode)
	}
}

// TestAdminRequiresAdmin confirms a normal user cannot reach admin routes.
func TestAdminRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // not an admin email
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if resp := h.get("/admin"); resp.StatusCode == http.StatusOK {
		t.Fatalf("non-admin reached /admin")
	} else {
		_ = resp.Body.Close()
	}
	if resp := h.post("/admin/users/create", url.Values{"email": {"x@x.com"}}); resp.StatusCode == http.StatusOK {
		t.Fatalf("non-admin reached create")
	} else {
		_ = body(t, resp)
	}
}
