package httpapp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// chrome is a marker that only the layout emits, so its presence in a response
// distinguishes a full page render from a fragment one.
const chrome = `class="topbar"`

// TestFragmentBranchOmitsLayout covers the three feed pages answering an
// HX-Target'd request with just that region: same markup, none of the layout.
func TestFragmentBranchOmitsLayout(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})

	cases := []struct{ name, path, target string }{
		{"activity", "/activity", "activity-feed"},
		{"friend", "/friends/2", "friend-feed"},
		{"group", "/groups/1", "group-feed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full := body(t, h.get(c.path))
			if !strings.Contains(full, `id="`+c.target+`"`) {
				t.Fatalf("full page missing the %s wrapper", c.target)
			}
			if !strings.Contains(full, chrome) {
				t.Fatalf("full page missing layout chrome")
			}

			resp := h.getHX(c.path, c.target)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("fragment status %d", resp.StatusCode)
			}
			frag := body(t, resp)
			if !strings.Contains(frag, `id="`+c.target+`"`) {
				t.Errorf("fragment missing its own wrapper: %s", frag)
			}
			if strings.Contains(frag, chrome) || strings.Contains(frag, "<!doctype") {
				t.Errorf("fragment leaked layout chrome: %s", frag)
			}
		})
	}
}

// TestBoostedRequestStillGetsFullPage guards the fallback: a boosted navigation
// is an htmx request too, but it targets the body and must not be answered with
// a bare fragment.
func TestBoostedRequestStillGetsFullPage(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	boosted := body(t, h.getHX("/activity", "body"))
	if !strings.Contains(boosted, chrome) {
		t.Errorf("boosted request did not get the full page: %s", boosted)
	}

	// A target we do not recognise falls through to the full page rather than
	// rendering nothing.
	other := body(t, h.getHX("/activity", "something-else"))
	if !strings.Contains(other, chrome) {
		t.Errorf("unknown target did not fall through to the full page")
	}
}

// TestPlainRequestIgnoresStrayTargetHeader covers the no-JS path: without
// HX-Request the target header is not htmx's and must not trigger a fragment.
func TestPlainRequestIgnoresStrayTargetHeader(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	req, err := http.NewRequest(http.MethodGet, h.srv.URL+"/activity", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Target", "activity-feed") // no HX-Request
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if b := body(t, resp); !strings.Contains(b, chrome) {
		t.Errorf("plain request was answered with a fragment: %s", b)
	}
}

// Rendering into a buffer means a template that cannot resolve its data now
// fails loudly instead of emitting a half-written page. This guards the case
// that found: the password-change error path used to re-render profile.html
// with nil data, so the user got a truncated page and the error was swallowed.
func TestErrorPathsRenderAUsablePage(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	resp := h.post("/profile/password", url.Values{
		"current": {"wrong-password"}, "password": {"newpassword123"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong current password status = %d, want 400", resp.StatusCode)
	}
	// The whole page must be there, not a prefix of it.
	if !strings.Contains(b, `name="language"`) {
		t.Error("error page is missing the language selector -- .Data did not resolve")
	}
	if !strings.Contains(b, "</html>") {
		t.Error("error page is truncated")
	}
}

// The freshness poller is what keeps a visible page under the staleness bar, so
// it must be present on pages that can be refreshed and absent on forms, where
// a timed re-render would discard what the user was typing.
func TestPollerOnlyOnPollablePages(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	for _, path := range []string{"/balances", "/friends", "/groups", "/activity", "/recurring"} {
		if b := body(t, h.get(path)); !strings.Contains(b, `class="poller"`) {
			t.Errorf("%s has no freshness poller", path)
		}
	}
	for _, path := range []string{"/expenses/new", "/profile", "/import", "/bank"} {
		if b := body(t, h.get(path)); strings.Contains(b, `class="poller"`) {
			t.Errorf("%s is a form page and must not be polled", path)
		}
	}
	// Logged out, there is nothing to keep fresh.
	h2 := newHarness(t)
	if b := body(t, h2.get("/login")); strings.Contains(b, `class="poller"`) {
		t.Error("login page emits a poller")
	}
}

// A poll asks for the content region and gets 204 when nothing moved -- that is
// what makes a 10s interval affordable.
func TestContentPollReturns204WhenUnchanged(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	first := h.getHX("/balances", "content")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first poll status = %d, want 200", first.StatusCode)
	}
	v := first.Header.Get("X-Fragment-Version")
	fb := body(t, first)
	if v == "" {
		t.Fatal("no X-Fragment-Version on the first poll")
	}
	if strings.Contains(fb, chrome) || strings.Contains(fb, "<!doctype") {
		t.Errorf("content fragment leaked layout chrome: %s", fb)
	}

	unchanged := h.getHX("/balances?v="+v, "content")
	if b := body(t, unchanged); b != "" {
		t.Errorf("204 carried a body: %s", b)
	}
	if unchanged.StatusCode != http.StatusNoContent {
		t.Errorf("unchanged poll status = %d, want 204", unchanged.StatusCode)
	}

	stale := h.getHX("/balances?v=deadbeef", "content")
	sb := body(t, stale)
	if stale.StatusCode != http.StatusOK || sb == "" {
		t.Errorf("stale poll got %d with %d bytes, want 200 with a body", stale.StatusCode, len(sb))
	}
}

// The 204 rests on the rendered block holding nothing per-request. Pages with
// CSRF-bearing forms are the case that would break it, so poll one twice: the
// double-submit token is per-session, and the fingerprint must not move.
func TestPollIsStableOnPagesWithForms(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	for _, path := range []string{"/recurring", "/groups", "/friends"} {
		first := h.getHX(path, "content")
		v1 := first.Header.Get("X-Fragment-Version")
		b := body(t, first)
		if !strings.Contains(b, "csrf_token") {
			t.Logf("%s rendered no CSRF form; still checking stability", path)
		}
		second := h.getHX(path, "content")
		v2 := second.Header.Get("X-Fragment-Version")
		_ = body(t, second)
		if v1 == "" || v1 != v2 {
			t.Errorf("%s fingerprint moved with no data change: %q -> %q", path, v1, v2)
		}
	}
}

// A real change must break the fingerprint, or the page would sit stale behind
// a stream of 204s -- the exact failure the freshness requirement forbids.
func TestContentVersionChangesWhenDataChanges(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	before := h.getHX("/friends", "content")
	v1 := before.Header.Get("X-Fragment-Version")
	_ = body(t, before)

	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	after := h.getHX("/friends", "content")
	v2 := after.Header.Get("X-Fragment-Version")
	_ = body(t, after)

	if v1 == "" || v2 == "" {
		t.Fatal("missing fragment version header")
	}
	if v1 == v2 {
		t.Error("adding a friend did not change the content fingerprint")
	}
}

// Render cost is published so a page load doubles as a profiling sample, but
// never on the credential pages.
func TestServerTimingHeader(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	resp := h.get("/balances")
	_ = body(t, resp)
	if st := resp.Header.Get("Server-Timing"); !strings.Contains(st, "render;dur=") {
		t.Errorf("Server-Timing = %q, want a render timing", st)
	}

	h2 := newHarness(t)
	login := h2.get("/login")
	_ = body(t, login)
	if st := login.Header.Get("Server-Timing"); st != "" {
		t.Errorf("login page published timing %q", st)
	}
}

// TestFlashShownOnceThenGone pins the behaviour the flash cookie exists for:
// the message rides the redirect, renders on the page it lands on, and does not
// come back on a refresh. The old ?flash= query param replayed it forever.
func TestFlashShownOnceThenGone(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123") // AdminEmails -> ADMIN
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	saved := body(t, h.post("/admin/users/2", url.Values{
		"name": {"Bobby"}, "email": {"bob@example.com"},
		"role": {"USER"}, "currency": {"USD"}, "language": {"en"},
	}))
	if !strings.Contains(saved, "User updated") {
		t.Fatalf("flash not shown on the redirected-to page: %s", saved)
	}

	if again := body(t, h.get("/admin")); strings.Contains(again, "User updated") {
		t.Errorf("flash replayed on refresh")
	}
}

// TestFlashNotInRedirectURL guards the specific regression: the confirmation
// must not be pushed into the address bar, where a refresh or a shared link
// would carry it.
func TestFlashNotInRedirectURL(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/admin/users/2", url.Values{
		"name": {"Bobby"}, "email": {"bob@example.com"},
		"role": {"USER"}, "currency": {"USD"}, "language": {"en"},
	})
	_ = body(t, resp)

	loc := resp.Header.Get("Location")
	if strings.Contains(loc, "flash=") {
		t.Errorf("redirect Location still carries the flash: %q", loc)
	}
	if loc != "/admin" {
		t.Errorf("redirect Location = %q, want /admin", loc)
	}
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == flashCookie && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("redirect did not set the flash cookie")
	}
}

// TestFlashCookieRoundTrip covers the encoding directly: a message with spaces
// and punctuation is not a legal raw cookie value, so it must survive base64.
func TestFlashCookieRoundTrip(t *testing.T) {
	const msg = `Archived 3 items, "done".`
	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.app.setFlash(rec, msg)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	if got := h.app.takeFlash(httptest.NewRecorder(), req); got != msg {
		t.Errorf("flash round trip = %q, want %q", got, msg)
	}
}

// The flash cookie must carry the same Secure flag as the session and CSRF
// cookies; it was the one cookie in the app that used to omit it.
func TestFlashCookieTracksSecureFlag(t *testing.T) {
	h := newHarness(t)
	h.app.Auth.Secure = true

	rec := httptest.NewRecorder()
	h.app.setFlash(rec, "hello")
	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == flashCookie {
			found = true
			if !c.Secure {
				t.Error("flash cookie is not Secure while the session cookie is")
			}
			if !c.HttpOnly {
				t.Error("flash cookie is not HttpOnly")
			}
		}
	}
	if !found {
		t.Fatal("setFlash wrote no cookie")
	}

	// The expiring cookie must match, or the browser treats it as a different
	// cookie and the original is never cleared.
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	clr := httptest.NewRecorder()
	_ = h.app.takeFlash(clr, req)
	for _, c := range clr.Result().Cookies() {
		if c.Name == flashCookie && !c.Secure {
			t.Error("flash-clearing cookie is not Secure")
		}
	}
}

// TestTakeFlashClearsCookie asserts the read expires the cookie in the same
// response -- that is what makes it one-shot.
func TestTakeFlashClearsCookie(t *testing.T) {
	h := newHarness(t)
	set := httptest.NewRecorder()
	h.app.setFlash(set, "hello")
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range set.Result().Cookies() {
		req.AddCookie(c)
	}

	cleared := httptest.NewRecorder()
	_ = h.app.takeFlash(cleared, req)
	var expired bool
	for _, c := range cleared.Result().Cookies() {
		if c.Name == flashCookie && c.MaxAge < 0 {
			expired = true
		}
	}
	if !expired {
		t.Error("takeFlash did not expire the cookie")
	}

	// No cookie at all is simply no flash, not an error.
	bare, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.app.takeFlash(httptest.NewRecorder(), bare); got != "" {
		t.Errorf("takeFlash with no cookie = %q, want empty", got)
	}

	// A cookie is client-controlled input: garbage in it must yield no message,
	// not a decode error or raw bytes rendered into the page.
	junk, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	junk.AddCookie(&http.Cookie{Name: flashCookie, Value: "!!!not-base64!!!"})
	if got := h.app.takeFlash(httptest.NewRecorder(), junk); got != "" {
		t.Errorf("takeFlash with a corrupt cookie = %q, want empty", got)
	}
}

// A background refresh or a freshness poll renders a page nobody has looked at
// yet, so it must leave a pending flash alone for the next real navigation.
func TestBackgroundRequestDoesNotConsumeFlash(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	// Queue a flash without following the redirect, so it is still pending.
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/admin/users/2", url.Values{
		"name": {"Bobby"}, "email": {"bob@example.com"},
		"role": {"USER"}, "currency": {"USD"}, "language": {"en"},
	})
	_ = body(t, resp)
	h.client.CheckRedirect = nil

	// A background render must not show or consume it...
	req, err := http.NewRequest(http.MethodGet, h.srv.URL+"/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(backgroundHeader, "1")
	bg, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if b := body(t, bg); strings.Contains(b, "User updated") {
		t.Error("background render consumed the pending flash")
	}

	// ...so the next real navigation still gets it.
	if b := body(t, h.get("/admin")); !strings.Contains(b, "User updated") {
		t.Error("flash was lost to the background render")
	}
}
