package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestThemeColorUpdate covers the theme-color happy path and the validation
// boundary: a valid slug persists and re-renders on every page, an invalid slug
// is rejected with 400 and does not change the stored value.
func TestThemeColorUpdate(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	// Fresh users default to burgundy.
	if b := body(t, h.get("/balances")); !strings.Contains(b, `data-accent="burgundy"`) {
		t.Fatalf("default theme not burgundy: %s", firstTag(b))
	}

	// Valid theme persists and shows up as the root data-accent everywhere.
	if resp := h.post("/profile", url.Values{"theme": {"teal"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("valid theme post status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	if b := body(t, h.get("/balances")); !strings.Contains(b, `data-accent="teal"`) {
		t.Fatalf("theme not applied after update: %s", firstTag(b))
	}

	// Invalid theme is rejected with 400 and leaves the stored value unchanged.
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/profile", url.Values{"theme": {"chartreuse"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid theme status = %d, want 400", resp.StatusCode)
	}
	_ = body(t, resp)
	h.client.CheckRedirect = nil
	if b := body(t, h.get("/balances")); !strings.Contains(b, `data-accent="teal"`) {
		t.Fatalf("invalid theme changed stored value: %s", firstTag(b))
	}
}

func firstTag(html string) string {
	if i := strings.Index(html, "<html"); i >= 0 {
		if j := strings.IndexByte(html[i:], '>'); j >= 0 {
			return html[i : i+j+1]
		}
	}
	return "<no html tag>"
}
