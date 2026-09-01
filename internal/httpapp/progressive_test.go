package httpapp

import (
	"net/url"
	"strings"
	"testing"
)

// TestLayoutClientWiring pins the htmx configuration the whole UI depends on.
// These are single attributes that silently degrade the app if dropped: without
// the extension script, hx-ext="morph" makes every swap a no-op.
func TestLayoutClientWiring(t *testing.T) {
	h := newHarness(t)
	page := body(t, h.get("/login"))

	for _, want := range []string{
		`hx-boost="true"`,
		`hx-ext="morph"`,
		`hx-swap="morph:innerHTML"`,
		"idiomorph-ext.min.js",
		`name="htmx-config"`,
		"globalViewTransitions",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("layout missing %q", want)
		}
	}

	// The extension must load after htmx itself, since it registers against it.
	if strings.Index(page, "htmx.min.js") > strings.Index(page, "idiomorph-ext.min.js") {
		t.Error("idiomorph loads before htmx; defineExtension would fail")
	}
}

// TestVendoredAssetsServed guards against the asset being referenced but not
// embedded -- the template would render a URL that 404s.
func TestVendoredAssetsServed(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/static/idiomorph-ext.min.js", "/static/htmx.min.js"} {
		resp := h.get(path)
		b := body(t, resp)
		if resp.StatusCode != 200 {
			t.Errorf("GET %s = %d", path, resp.StatusCode)
		}
		if len(b) == 0 {
			t.Errorf("GET %s served an empty body", path)
		}
	}
	// The stray editor-backup directory must not be embedded or reachable.
	resp := h.get("/static/nppBackup/icon.svg.2026-07-13_162951.bak")
	_ = body(t, resp)
	if resp.StatusCode == 200 {
		t.Error("editor backup files are still embedded and served under /static/")
	}
}

// TestDoubleSubmitGuards covers the mutating forms carrying hx-disabled-elt, so
// a fast double-tap on mobile cannot fire the same POST twice.
func TestDoubleSubmitGuards(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	const guard = `hx-disabled-elt="find button[type=submit]"`
	for _, page := range []string{"/expenses/new", "/groups", "/friends", "/recurring", "/import"} {
		if b := body(t, h.get(page)); !strings.Contains(b, guard) {
			t.Errorf("%s has a mutating form with no double-submit guard", page)
		}
	}
}

// TestAmountPatternValidation covers the client-side amount check: htmx runs
// HTML validation before a boosted submit, so a mistyped amount never costs a
// round trip. The pattern must accept what money.Parse accepts.
func TestAmountPatternValidation(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	form := body(t, h.get("/expenses/new"))
	if !strings.Contains(form, `pattern="\s*[+-]?[0-9,]*\.?[0-9]*\s*"`) {
		t.Error("amount input missing the client-side format pattern")
	}
}

// TestBoostOptOuts covers the links and forms that must NOT be htmx-boosted:
// a file download needs a real navigation for Content-Disposition to apply, and
// the auth forms deliberately do a full page POST.
func TestBoostOptOuts(t *testing.T) {
	h := newHarness(t)

	if b := body(t, h.get("/forgot-password")); !strings.Contains(b, `hx-boost="false"`) {
		t.Error("forgot-password form is boosted; siblings login/register are not")
	}
	if b := body(t, h.get("/reset-password?token=abc")); !strings.Contains(b, `hx-boost="false"`) {
		t.Error("reset-password form is boosted; siblings login/register are not")
	}

	h.register("Alice", "alice@example.com", "password123")
	profile := body(t, h.get("/profile"))
	i := strings.Index(profile, `href="/profile/export"`)
	if i < 0 {
		t.Fatal("profile page missing the export link")
	}
	// The opt-out has to be on the export anchor itself, not merely elsewhere
	// on the page, or htmx swallows the download.
	if end := strings.Index(profile[i:], ">"); end < 0 || !strings.Contains(profile[i:i+end], `hx-boost="false"`) {
		t.Error("export link is boosted; htmx would swap the file into the page")
	}
}

// TestExpenseRowOptsOutOfFragmentTarget guards the one override inside the feed:
// rows are navigation, so they must swap the page, not the feed they live in.
func TestExpenseRowOptsOutOfFragmentTarget(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2025-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {""},
		"include_2": {"1"}, "value_2": {""},
	})

	feed := body(t, h.get("/activity"))
	i := strings.Index(feed, `class="l-row`)
	if i < 0 {
		t.Fatal("activity feed rendered no expense rows")
	}
	end := strings.Index(feed[i:], ">")
	if end < 0 {
		t.Fatal("malformed row markup")
	}
	if !strings.Contains(feed[i:i+end], `hx-target="body"`) {
		t.Errorf("expense row would swap into the feed instead of navigating: %s", feed[i:i+end])
	}
}
