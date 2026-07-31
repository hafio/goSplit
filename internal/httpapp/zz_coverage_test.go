package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// noRedirect stops the client from following 3xx so a test can assert on the
// redirect itself. It mirrors the inline closure used across the package's tests.
func covNoRedirect(h *harness) {
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
}

// --- friends: not-found + hide/delete ------------------------------------

// TestFriendDetailNotFound covers handleFriendDetail's two 404 branches: a
// non-numeric id (atoi64 fails) and a numeric id with no such user.
func TestFriendDetailNotFound(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	for _, path := range []string{"/friends/abc", "/friends/9999"} {
		resp := h.get(path)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status %d, want 404", path, resp.StatusCode)
		}
	}
}

// TestFriendHideAndDelete exercises handleFriendHide (a hidden friend still
// appears in the list) and handleFriendDelete (the friendship is removed, so the
// friend drops off the page).
func TestFriendHideAndDelete(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2

	// Hide redirects back to /friends (followed -> 200); Bob is still listed.
	if resp := h.post("/friends/2/hide", url.Values{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("hide status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	if b := body(t, h.get("/friends")); !strings.Contains(b, "bob@example.com") {
		t.Error("hidden friend should still appear on the friends page")
	}

	// Delete removes the friendship entirely.
	if resp := h.post("/friends/2/delete", url.Values{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	if b := body(t, h.get("/friends")); strings.Contains(b, "bob@example.com") {
		t.Error("deleted friend still listed")
	}
}

// --- groups: not-found, forbidden, join flow ------------------------------

// TestGroupDetailNotFoundAndForbidden covers handleGroupDetail's 404 branches
// (bad id, missing group) and its 403 branch (a logged-in non-member).
func TestGroupDetailNotFoundAndForbidden(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}}) // group 1

	for _, path := range []string{"/groups/abc", "/groups/9999"} {
		resp := h.get(path)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status %d, want 404", path, resp.StatusCode)
		}
	}

	// Carol is a fresh account and not a member of group 1.
	h.register("Carol", "carol@example.com", "password123")
	resp := h.get("/groups/1")
	if b := body(t, resp); resp.StatusCode != http.StatusForbidden || !strings.Contains(b, "forbidden") {
		t.Errorf("non-member GET /groups/1 status %d, want 403 forbidden", resp.StatusCode)
	}
}

// TestGroupJoinFlow covers handleGroupJoinPage (render + 404) and handleGroupJoin
// (a public-id join makes the caller a member).
func TestGroupJoinFlow(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}}) // group 1

	var pid string
	if err := h.st.DB.QueryRow(`SELECT public_id FROM groups WHERE id = 1`).Scan(&pid); err != nil {
		t.Fatalf("look up public_id: %v", err)
	}

	// Unknown public id -> 404.
	if resp := h.get("/g/does-not-exist"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /g/does-not-exist status %d, want 404", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// Bob is a separate account; the join page renders and the join adds him.
	h.register("Bob", "bob@example.com", "password123")
	if resp := h.get("/g/" + pid); resp.StatusCode != http.StatusOK {
		t.Fatalf("join page status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	if resp := h.post("/groups/join/"+pid, url.Values{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("join post status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	// Bob is now a member, so the group detail page is reachable (no 403).
	if resp := h.get("/groups/1"); resp.StatusCode != http.StatusOK {
		t.Errorf("joined member GET /groups/1 status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// --- expenses: detail 404 + friend-context add form -----------------------

// TestExpenseDetailNotFound covers handleExpenseDetail's 404 for an unknown id.
func TestExpenseDetailNotFound(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	resp := h.get("/expenses/nope")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /expenses/nope status %d, want 404", resp.StatusCode)
	}
}

// TestExpenseNewFriendContext exercises candidatesForContext's friend branch: the
// friend is offered as a participant on the add form.
func TestExpenseNewFriendContext(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2

	b := body(t, h.get("/expenses/new?friend=2"))
	if !strings.Contains(b, "bob@example.com") {
		t.Error("friend-context add form missing the friend as a candidate")
	}
}

// --- currency conversion: page render + input validation ------------------

// TestConvertPageRendersAndNotFound covers handleConvertPage: it renders for a
// real friend and 404s for a bad or unknown id.
func TestConvertPageRendersAndNotFound(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2

	if b := body(t, h.get("/friends/2/convert")); !strings.Contains(b, "bob@example.com") {
		t.Error("convert page missing the friend")
	}
	for _, path := range []string{"/friends/abc/convert", "/friends/9999/convert"} {
		resp := h.get(path)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status %d, want 404", path, resp.StatusCode)
		}
	}
}

// TestConvertRejectsBadInput covers handleConvert's three 400 branches: an
// unparseable from-amount, an unparseable to-amount, and an unknown direction.
func TestConvertRejectsBadInput(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2
	covNoRedirect(h)

	cases := []url.Values{
		{"from_currency": {"USD"}, "to_currency": {"EUR"}, "from_amount": {"abc"}, "to_amount": {"1.00"}, "direction": {"owed"}},
		{"from_currency": {"USD"}, "to_currency": {"EUR"}, "from_amount": {"1.00"}, "to_amount": {"xyz"}, "direction": {"owed"}},
		{"from_currency": {"USD"}, "to_currency": {"EUR"}, "from_amount": {"1.00"}, "to_amount": {"1.00"}, "direction": {"sideways"}},
	}
	for i, form := range cases {
		resp := h.post("/friends/2/convert", form)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("case %d status %d, want 400", i, resp.StatusCode)
		}
	}
}

// --- import + bank pages --------------------------------------------------

// TestImportPageRenders covers handleImportPage (GET render only; the POST path
// hits an external API and is out of scope here).
func TestImportPageRenders(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	if resp := h.get("/import"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /import status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// TestBankPageAndConvertNotFound covers handleBankPage (renders with banking
// disabled and no cached transactions) and handleBankConvert's 404 for an unknown
// transaction id.
func TestBankPageAndConvertNotFound(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	if resp := h.get("/bank"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /bank status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	resp := h.get("/bank/tx/unknown/convert")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /bank/tx/unknown/convert status %d, want 404", resp.StatusCode)
	}
}

// --- profile: page, update, password change -------------------------------

// TestProfilePageAndUpdate covers handleProfile (render) and handleProfileUpdate
// persisting the name field.
func TestProfilePageAndUpdate(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	if resp := h.get("/profile"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profile status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}

	if resp := h.post("/profile", url.Values{
		"name": {"Alicia"}, "currency": {"USD"}, "default_currency": {"EUR"}, "language": {"en"},
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("profile update status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}

	var name, defCur string
	if err := h.st.DB.QueryRow(`SELECT name, default_currency FROM users WHERE id = 1`).Scan(&name, &defCur); err != nil {
		t.Fatalf("re-read user: %v", err)
	}
	if name != "Alicia" {
		t.Errorf("name = %q, want Alicia", name)
	}
	if defCur != "EUR" {
		t.Errorf("default_currency = %q, want EUR", defCur)
	}
}

// TestPasswordChange covers handlePasswordChange: a wrong current password is
// rejected with 400, and a correct one clears the session and redirects to login.
func TestPasswordChange(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)

	// Wrong current password -> 400, session intact.
	resp := h.post("/profile/password", url.Values{"current": {"wrongpass99"}, "password": {"newpassword123"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong-current status %d, want 400", resp.StatusCode)
	}

	// Correct current password -> session cleared, redirect to /login.
	resp = h.post("/profile/password", url.Values{"current": {"password123"}, "password": {"newpassword123"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("password change status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Errorf("redirect location = %q, want /login prefix", loc)
	}
}

// TestExportUserData covers handleExport: a JSON attachment download.
func TestExportUserData(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	resp := h.get("/profile/export")
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "gosplit-export.json") {
		t.Errorf("Content-Disposition = %q, want the export filename", cd)
	}
	if len(b) == 0 {
		t.Error("export body is empty")
	}
}

// --- misc pages: offline, home redirects ----------------------------------

// TestOfflinePage covers handleOffline (reachable without a login).
func TestOfflinePage(t *testing.T) {
	h := newHarness(t)
	if resp := h.get("/offline"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /offline status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// TestHomeRedirects covers handleHome: anonymous -> /login, authenticated ->
// the configured default homepage (/balances).
func TestHomeRedirects(t *testing.T) {
	h := newHarness(t)
	covNoRedirect(h)

	resp := h.get("/")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("anon GET / status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Errorf("anon redirect = %q, want /login", loc)
	}

	h.client.CheckRedirect = nil
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)
	resp = h.get("/")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("authed GET / status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/balances" {
		t.Errorf("authed redirect = %q, want /balances", loc)
	}
}

// --- auth: forgot + reset pages -------------------------------------------

// TestForgotAndResetPages covers handleForgotPage, handleForgot (renders the
// check-email message via the LogMailer, no network), and handleResetPage's
// missing-token (400) and with-token (200) branches.
func TestForgotAndResetPages(t *testing.T) {
	h := newHarness(t)
	_ = body(t, h.get("/login")) // seed a CSRF cookie for the POST

	if resp := h.get("/forgot-password"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /forgot-password status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	if resp := h.post("/forgot-password", url.Values{"email": {"nobody@example.com"}}); resp.StatusCode != http.StatusOK {
		t.Errorf("POST /forgot-password status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	if resp := h.get("/reset-password"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET /reset-password (no token) status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	if resp := h.get("/reset-password?token=abc123"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /reset-password?token= status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// --- recurring: list + delete ---------------------------------------------

// TestRecurringListAndDelete covers handleRecurringList (renders with a recent
// expense offered as a template) and handleRecurringDelete (always redirects,
// even for an unknown id).
func TestRecurringListAndDelete(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-01-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})

	if b := body(t, h.get("/recurring")); !strings.Contains(b, "Dinner") {
		t.Error("recurring page missing the recent expense as a template option")
	}

	covNoRedirect(h)
	resp := h.post("/recurring/999/delete", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("recurring delete status %d, want 303", resp.StatusCode)
	}
}

// --- collapse/archive preview pages ---------------------------------------

// TestCollapsePreviewPages covers handleFriendCollapsePage and
// handleGroupCollapsePage (both with and without the ?before= preview) plus
// handleGroupCollapse's POST redirect.
func TestCollapsePreviewPages(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-01-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})

	for _, path := range []string{
		"/friends/2/collapse", "/friends/2/collapse?before=2026-03-01",
		"/groups/1/collapse", "/groups/1/collapse?before=2026-03-01",
	} {
		if resp := h.get(path); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status %d, want 200", path, resp.StatusCode)
			_ = body(t, resp)
		} else {
			_ = body(t, resp)
		}
	}

	// Group collapse applies and redirects back to the group (followed -> 200).
	if resp := h.post("/groups/1/collapse", url.Values{"before": {"2026-03-01"}}); resp.StatusCode != http.StatusOK {
		t.Errorf("group collapse POST status %d, want 200", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// --- admin: deactivate toggle ---------------------------------------------

// TestAdminToggle covers handleAdminToggle: toggling an existing user redirects,
// and an unknown id 404s.
func TestAdminToggle(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123") // AdminEmails -> ADMIN
	_ = body(t, h.post("/admin/users/create", url.Values{
		"email": {"victim@example.com"}, "name": {"Victim"}, "role": {"USER"},
		"currency": {"USD"}, "language": {"en"}, "password": {"password12"},
	})) // id 2

	if resp := h.post("/admin/users/2/toggle", url.Values{}); resp.StatusCode != http.StatusOK {
		t.Errorf("toggle status %d, want 200 (followed redirect)", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	covNoRedirect(h)
	resp := h.post("/admin/users/9999/toggle", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("toggle unknown id status %d, want 404", resp.StatusCode)
	}
}

// --- push: public key, test, subscribe/unsubscribe ------------------------

// TestPushPublicKeyDisabled covers handlePushPublicKey when push is not
// configured: the JSON reports enabled=false.
func TestPushPublicKeyDisabled(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	b := body(t, h.get("/push/public-key"))
	if !strings.Contains(b, `"enabled":false`) {
		t.Errorf("public-key body = %s, want enabled:false", b)
	}
}

// TestPushTestNotConfigured covers handlePushTest's guard: with push disabled it
// returns 503 before touching subscriptions.
func TestPushTestNotConfigured(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)
	resp := h.post("/push/test", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("push test status %d, want 503", resp.StatusCode)
	}
}

// TestPushSubscribeInvalid covers handlePushSubscribe's rejection of a body that
// is not a valid subscription (400) and handlePushUnsubscribe's tolerant 204.
func TestPushSubscribeInvalid(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)

	resp := h.post("/push/subscribe", url.Values{})
	if b := body(t, resp); resp.StatusCode != http.StatusBadRequest || !strings.Contains(b, "invalid subscription") {
		t.Errorf("subscribe status %d body %q, want 400 invalid subscription", resp.StatusCode, b)
	}

	resp = h.post("/push/unsubscribe", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("unsubscribe status %d, want 204", resp.StatusCode)
	}
}
