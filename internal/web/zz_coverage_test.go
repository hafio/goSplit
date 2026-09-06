package web

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// covRenderer builds a Renderer or fails the test. NewRenderer parses the
// embedded templates and loads i18n; both are baked into the binary so this
// must succeed.
func covRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	if r == nil {
		t.Fatal("NewRenderer() returned nil renderer")
	}
	return r
}

func TestCovCSVRows(t *testing.T) {
	// Empty / whitespace-only input returns nil.
	if got := csvRows(""); got != nil {
		t.Errorf("csvRows(\"\") = %v, want nil", got)
	}
	if got := csvRows("   \n  "); got != nil {
		t.Errorf("csvRows(whitespace) = %v, want nil", got)
	}
	// Valid CSV parses into rows.
	got := csvRows("a,b\nc,d")
	want := [][]string{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("csvRows valid = %v, want %v", got, want)
	}
	// Ragged rows (mismatched field counts) fail the reader and return nil.
	if got := csvRows("a,b\nc"); got != nil {
		t.Errorf("csvRows(ragged) = %v, want nil", got)
	}
}

func TestCovMethodGlyph(t *testing.T) {
	// ASCII glyph mappings with exact expected values.
	ascii := map[string]string{
		"EQUAL":      "=",
		"EXACT":      "1.23",
		"PERCENTAGE": "%",
		"equal":      "=", // case-insensitive via ToUpper
	}
	for in, want := range ascii {
		if got := methodGlyph(in); got != want {
			t.Errorf("methodGlyph(%q) = %q, want %q", in, got, want)
		}
	}
	// SHARE and ADJUSTMENT map to non-ASCII glyphs; assert only that a mapping
	// happened (non-empty and different from the input code) to keep this file
	// ASCII-only.
	for _, code := range []string{"SHARE", "ADJUSTMENT"} {
		got := methodGlyph(code)
		if got == "" || got == code {
			t.Errorf("methodGlyph(%q) = %q, want a mapped glyph", code, got)
		}
	}
	// Unknown codes are returned unchanged (the original, un-uppercased value).
	if got := methodGlyph("weird"); got != "weird" {
		t.Errorf("methodGlyph(weird) = %q, want weird", got)
	}
}

func TestCovCurrencyCodes(t *testing.T) {
	cs := currencyCodes()
	if len(cs) == 0 {
		t.Fatal("currencyCodes() empty")
	}
	seen := map[string]bool{}
	for _, c := range cs {
		seen[c] = true
	}
	for _, want := range []string{"USD", "EUR", "GBP"} {
		if !seen[want] {
			t.Errorf("currencyCodes() missing %q", want)
		}
	}
}

func TestCovFuncsAbs64(t *testing.T) {
	fn, ok := funcs["abs64"].(func(int64) int64)
	if !ok {
		t.Fatal("funcs[abs64] is not func(int64) int64")
	}
	cases := map[int64]int64{-5: 5, 5: 5, 0: 0}
	for in, want := range cases {
		if got := fn(in); got != want {
			t.Errorf("abs64(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestCovFuncsDict(t *testing.T) {
	fn, ok := funcs["dict"].(func(...any) map[string]any)
	if !ok {
		t.Fatal("funcs[dict] is not func(...any) map[string]any")
	}
	m := fn("a", 1, "b", 2)
	if len(m) != 2 || m["a"] != 1 || m["b"] != 2 {
		t.Errorf("dict(a,1,b,2) = %v", m)
	}
	// A trailing key with no value is dropped (the i+1 < len guard).
	m2 := fn("a", 1, "orphan")
	if len(m2) != 1 || m2["a"] != 1 {
		t.Errorf("dict(a,1,orphan) = %v, want only a:1", m2)
	}
	// No args yields an empty (non-nil) map.
	if m3 := fn(); m3 == nil || len(m3) != 0 {
		t.Errorf("dict() = %v, want empty map", m3)
	}
}

func TestCovRendererT(t *testing.T) {
	r := covRenderer(t)
	// An unknown key falls back to the key itself.
	const missing = "___cov_missing_key___"
	if got := r.T(DefaultTheme, missing); got != missing {
		t.Errorf("Renderer.T(unknown) = %q, want %q", got, missing)
	}
}

func TestCovRendererLanguages(t *testing.T) {
	r := covRenderer(t)
	langs := r.Languages()
	if len(langs) == 0 {
		t.Fatal("Languages() empty")
	}
	// DetectLang with no preference and no Accept-Language resolves to a locale
	// that is one of the advertised languages.
	got := r.DetectLang("", "")
	found := false
	for _, l := range langs {
		if l == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DetectLang(\"\",\"\") = %q, not in Languages() %v", got, langs)
	}
}

func TestCovViewDataT(t *testing.T) {
	// A zero-value ViewData has a nil bundle and returns the key unchanged.
	var vd ViewData
	if got := vd.T("some.key"); got != "some.key" {
		t.Errorf("ViewData.T with nil bundle = %q, want %q", got, "some.key")
	}
	// With a bundle, an unknown key still falls back to the key itself.
	r := covRenderer(t)
	vd2 := ViewData{Lang: "", bundle: r.bundle}
	const missing = "___cov_vd_missing___"
	if got := vd2.T(missing); got != missing {
		t.Errorf("ViewData.T(unknown) = %q, want %q", got, missing)
	}
}

func TestCovRenderUnknownPage(t *testing.T) {
	r := covRenderer(t)
	rec := httptest.NewRecorder()
	r.Render(rec, http.StatusOK, "no_such_page", ViewData{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Render(unknown page) status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body := rec.Body.String(); !strings.Contains(body, "unknown page: no_such_page") {
		t.Errorf("Render(unknown page) body = %q, want unknown-page message", body)
	}
}

func TestCovRenderKnownPage(t *testing.T) {
	r := covRenderer(t)
	// Empty Lang and an invalid Theme exercise the defaulting branches. The page
	// is rendered into a buffer first, so a template failure would surface as a
	// 500 here rather than a half-written 200.
	rec := httptest.NewRecorder()
	r.Render(rec, http.StatusOK, "login", ViewData{Lang: "", Theme: "not-a-real-theme"})
	if rec.Code != http.StatusOK {
		t.Errorf("Render(login) status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Render(login) Content-Type = %q", ct)
	}
	// Pages must never be cached client-side.
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Render(login) Cache-Control = %q, want no-store", cc)
	}
	if p := rec.Header().Get("Pragma"); p != "no-cache" {
		t.Errorf("Render(login) Pragma = %q, want no-cache", p)
	}
	if e := rec.Header().Get("Expires"); e != "0" {
		t.Errorf("Render(login) Expires = %q, want 0", e)
	}
}

func TestCovAssetsHandlerCacheControl(t *testing.T) {
	r := covRenderer(t)
	h := r.AssetsHandler()

	// Fingerprinted request (?v=...) gets the immutable, long-lived cache.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, staticPrefix+"app.css?v=abc123", nil)
	h.ServeHTTP(rec, req)
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("versioned Cache-Control = %q", cc)
	}

	// Unversioned request gets the short cache.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, staticPrefix+"app.css", nil)
	h.ServeHTTP(rec2, req2)
	if cc := rec2.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("unversioned Cache-Control = %q", cc)
	}
}

func TestCovAsset(t *testing.T) {
	// A known embedded asset returns non-empty bytes and no error.
	b, err := Asset("app.css")
	if err != nil {
		t.Fatalf("Asset(app.css) error = %v", err)
	}
	if len(b) == 0 {
		t.Error("Asset(app.css) returned empty bytes")
	}
	// A missing asset returns an error.
	if _, err := Asset("___cov_missing_asset___.css"); err == nil {
		t.Error("Asset(missing) error = nil, want error")
	}
}
