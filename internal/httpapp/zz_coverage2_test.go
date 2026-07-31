package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// covPostJSON posts a raw JSON body with the double-submit CSRF token carried in
// the X-CSRF-Token header (VerifyCSRF accepts either the header or the
// csrf_token form field). The harness's post() helper only sends form bodies, so
// JSON endpoints (push subscribe) need this.
func covPostJSON(h *harness, path, jsonBody string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, strings.NewReader(jsonBody))
	if err != nil {
		h.t.Fatalf("build JSON POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf())
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("JSON POST %s: %v", path, err)
	}
	return resp
}

// --- auth: register page, login errors, safeNext --------------------------

// TestCov2RegisterPageRenders covers handleRegisterPage's normal render (signup
// enabled) -- the harness registers via POST directly, so the GET was untested.
func TestCov2RegisterPageRenders(t *testing.T) {
	h := newHarness(t)
	resp := h.get("/register")
	b := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /register status %d, want 200", resp.StatusCode)
	}
	if len(b) == 0 {
		t.Error("register page body is empty")
	}
}

// TestCov2LoginBadCredentials covers handleLogin's LoginPassword error branch:
// wrong credentials render the login page with 401.
func TestCov2LoginBadCredentials(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	_ = body(t, h.get("/login")) // ensure CSRF cookie
	resp := h.post("/login", url.Values{"email": {"alice@example.com"}, "password": {"wrongpass99"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-credentials login status %d, want 401", resp.StatusCode)
	}
}

// TestCov2RegisterDuplicate covers handleRegister's error branch: registering an
// email that already exists is rejected with 400.
func TestCov2RegisterDuplicate(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	resp := h.post("/register", url.Values{"name": {"Alice2"}, "email": {"alice@example.com"}, "password": {"password123"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate register status %d, want 400", resp.StatusCode)
	}
}

// TestCov2LoginNextRedirect covers safeNext's "return next" branch: a local
// ?next path is honored after a successful login.
func TestCov2LoginNextRedirect(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)
	resp := h.post("/login?next=/groups", url.Values{"email": {"alice@example.com"}, "password": {"password123"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/groups" {
		t.Errorf("redirect location = %q, want /groups", loc)
	}
}

// --- auth: reset + magic-link full flows ----------------------------------

// TestCov2ResetPasswordFlow covers handleReset: a real reset token (minted via
// forgot-password over the LogMailer) drives the success path, and a bogus token
// hits the 400 branch.
func TestCov2ResetPasswordFlow(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	// Mint a reset token (LogMailer, no network).
	if resp := h.post("/forgot-password", url.Values{"email": {"alice@example.com"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("forgot-password status %d, want 200", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	var token string
	if err := h.st.DB.QueryRow(`SELECT token FROM verification_tokens WHERE purpose = 'reset'`).Scan(&token); err != nil {
		t.Fatalf("look up reset token: %v", err)
	}

	// Bad token -> 400.
	if resp := h.post("/reset-password", url.Values{"token": {"not-a-real-token"}, "password": {"newpassword123"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad-token reset status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// Valid token -> success message (200).
	resp := h.post("/reset-password", url.Values{"token": {token}, "password": {"newpassword123"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid-token reset status %d, want 200", resp.StatusCode)
	}
}

// TestCov2MagicLinkFlow covers handleMagicRequest (empty-email 400 and a
// successful request over the LogMailer) plus handleMagicConsume (bad token 400
// and a valid token that starts a session and redirects home).
func TestCov2MagicLinkFlow(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	// Empty email -> RequestMagicLink returns "email is required" -> 400.
	if resp := h.post("/auth/magic", url.Values{"email": {""}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty-email magic status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// Valid request for the existing (active) user mints a magic token.
	if resp := h.post("/auth/magic", url.Values{"email": {"alice@example.com"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("magic request status %d, want 200", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	var token string
	if err := h.st.DB.QueryRow(`SELECT token FROM verification_tokens WHERE purpose = 'magic'`).Scan(&token); err != nil {
		t.Fatalf("look up magic token: %v", err)
	}

	covNoRedirect(h)
	// Bad token -> 400 message page.
	if resp := h.get("/auth/magic?token=not-a-real-token"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad magic-consume status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	// Valid token -> session started, redirect home.
	resp := h.get("/auth/magic?token=" + token)
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("valid magic-consume status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Errorf("magic-consume redirect = %q, want /", loc)
	}
}

// --- recurring: create success + list rows + bad-cron error ---------------

// TestCov2RecurringCreateAndList covers handleRecurringCreate (success redirect
// and the bad-cron 400 branch) and the handleRecurringList row-rendering loop,
// which the round-1 test left untested because no recurrence existed.
func TestCov2RecurringCreateAndList(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Groceries"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-01-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	var expID string
	if err := h.st.DB.QueryRow(`SELECT id FROM expenses WHERE name = 'Groceries'`).Scan(&expID); err != nil {
		t.Fatalf("look up expense id: %v", err)
	}

	// Bad cron -> CreateRecurrence error -> 400.
	if resp := h.post("/recurring", url.Values{"template_id": {expID}, "cron": {"not a cron"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad-cron recurrence status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// Valid cron -> success redirect (followed -> 200).
	if resp := h.post("/recurring", url.Values{"template_id": {expID}, "cron": {"0 9 * * *"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create recurrence status %d, want 200 (followed redirect)", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}

	// The list now renders a recurrence row resolving the template's name.
	if b := body(t, h.get("/recurring")); !strings.Contains(b, "Groceries") {
		t.Error("recurring page missing the scheduled recurrence's template name")
	}
}

// --- currency conversion: success legs + preselected balances -------------

// TestCov2ConvertSuccess covers handleConvert's success path for both
// directions: a valid conversion creates the expense pair and redirects back to
// the friend.
func TestCov2ConvertSuccess(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2
	covNoRedirect(h)

	for _, dir := range []string{"owed", "owe"} {
		form := url.Values{
			"from_currency": {"USD"}, "to_currency": {"EUR"},
			"from_amount": {"10.00"}, "to_amount": {"9.00"}, "direction": {dir},
			"date": {"2026-02-01"},
		}
		resp := h.post("/friends/2/convert", form)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("convert (%s) status %d, want 303", dir, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/friends/2") {
			t.Errorf("convert (%s) redirect = %q, want /friends/2 prefix", dir, loc)
		}
	}
}

// TestCov2ConvertPagePreselectsBalance covers handleConvertPage's nonzero-balance
// preselection loop (break) and absInt64 for both signs: a friend who owes the
// viewer (positive) and one the viewer owes (negative -> direction "owe").
func TestCov2ConvertPagePreselectsBalance(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})  // id 2
	h.post("/friends/add", url.Values{"email": {"dave@example.com"}}) // id 3

	// Bob owes Alice: Alice pays a direct expense (positive balance -> absInt64 v).
	h.post("/expenses", url.Values{
		"name": {"BobDinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-05"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	// Alice owes Dave: Dave pays (negative balance -> absInt64 -v, direction owe).
	h.post("/expenses", url.Values{
		"name": {"DaveDinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-06"}, "method": {"EQUAL"}, "paid_by": {"3"},
		"include_1": {"1"}, "include_3": {"1"},
	})

	for _, path := range []string{"/friends/2/convert", "/friends/3/convert"} {
		if resp := h.get(path); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status %d, want 200", path, resp.StatusCode)
			_ = body(t, resp)
		} else {
			_ = body(t, resp)
		}
	}
}

// --- archive/collapse: friend collapse, error, and bad-id 404s ------------

// TestCov2FriendCollapse covers handleFriendCollapse (a real cutoff collapses the
// direct history and redirects; a cutoff with nothing before it hits the
// ErrNothingToArchive 400) and the bad-id / not-found 404 branches on both the
// friend and group collapse routes.
func TestCov2FriendCollapse(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // id 2
	// A 2-person direct expense dated in the past is collapsible.
	h.post("/expenses", url.Values{
		"name": {"OldDinner"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2020-01-01"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})

	// Nothing before 1900 -> ErrNothingToArchive -> 400.
	covNoRedirect(h)
	if resp := h.post("/friends/2/collapse", url.Values{"before": {"1900-01-01"}}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty friend collapse status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}

	// Real cutoff collapses the 2020 expense and redirects to the friend.
	resp := h.post("/friends/2/collapse", url.Values{"before": {"2021-01-01"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("friend collapse status %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/friends/2") {
		t.Errorf("friend collapse redirect = %q, want /friends/2 prefix", loc)
	}

	// Bad-id / not-found 404 branches across the collapse handlers.
	notFound := []struct {
		method, path string
	}{
		{http.MethodGet, "/friends/abc/collapse"},
		{http.MethodGet, "/friends/9999/collapse"},
		{http.MethodGet, "/groups/abc/collapse"},
		{http.MethodGet, "/groups/9999/collapse"},
		{http.MethodPost, "/friends/abc/collapse"},
		{http.MethodPost, "/groups/abc/collapse"},
	}
	for _, tc := range notFound {
		var r *http.Response
		if tc.method == http.MethodGet {
			r = h.get(tc.path)
		} else {
			r = h.post(tc.path, url.Values{})
		}
		_ = body(t, r)
		if r.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s status %d, want 404", tc.method, tc.path, r.StatusCode)
		}
	}
}

// TestCov2GroupCollapseError covers handleGroupCollapse's ErrNothingToArchive
// 400 branch: a member collapsing a group with no history before the cutoff.
func TestCov2GroupCollapseError(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}}) // group 1
	covNoRedirect(h)
	resp := h.post("/groups/1/collapse", url.Values{"before": {"1900-01-01"}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty group collapse status %d, want 400", resp.StatusCode)
	}
}

// --- bank: disabled-provider error branches -------------------------------

// TestCov2BankDisabledErrors covers the error branches of the bank handlers when
// no provider is configured (the harness default): link-token 503, exchange 400,
// and sync 400 (no account connected).
func TestCov2BankDisabledErrors(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)

	if resp := h.post("/bank/link-token", url.Values{}); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("link-token status %d, want 503", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	if resp := h.post("/bank/exchange", url.Values{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("exchange status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	if resp := h.post("/bank/sync", url.Values{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("sync status %d, want 400", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
}

// --- import: missing api-key error branch ---------------------------------

// TestCov2ImportSplitwiseMissingKey covers handleImportSplitwise's error branch:
// an empty API key is rejected (400) before any network call is attempted.
func TestCov2ImportSplitwiseMissingKey(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	resp := h.post("/import/splitwise", url.Values{"api_key": {""}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty-key import status %d, want 400", resp.StatusCode)
	}
}

// --- push: successful subscribe -------------------------------------------

// TestCov2PushSubscribeSuccess covers handlePushSubscribe's success path: a valid
// JSON subscription is stored and returns 204.
func TestCov2PushSubscribeSuccess(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	resp := covPostJSON(h, "/push/subscribe", `{"endpoint":"https://push.example.com/abc","keys":{"p256dh":"x","auth":"y"}}`)
	_ = body(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("push subscribe status %d, want 204", resp.StatusCode)
	}
}
