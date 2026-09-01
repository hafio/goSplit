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
	rec := httptest.NewRecorder()
	setFlash(rec, msg)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	if got := takeFlash(httptest.NewRecorder(), req); got != msg {
		t.Errorf("flash round trip = %q, want %q", got, msg)
	}
}

// TestTakeFlashClearsCookie asserts the read expires the cookie in the same
// response -- that is what makes it one-shot.
func TestTakeFlashClearsCookie(t *testing.T) {
	set := httptest.NewRecorder()
	setFlash(set, "hello")
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range set.Result().Cookies() {
		req.AddCookie(c)
	}

	cleared := httptest.NewRecorder()
	_ = takeFlash(cleared, req)
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
	if got := takeFlash(httptest.NewRecorder(), bare); got != "" {
		t.Errorf("takeFlash with no cookie = %q, want empty", got)
	}
}
