package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// feedViewData is the minimum a feed page needs to render: an empty filter map
// and no rows, which exercises the empty-state branch of every shared partial.
func feedViewData() ViewData {
	return ViewData{Data: map[string]any{
		"Filter": map[string]string{}, "Items": nil, "ShowAllHref": "",
	}}
}

func TestRenderFragmentOmitsLayout(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	rec := httptest.NewRecorder()
	r.RenderFragment(rec, http.StatusOK, "activity", "frag_activity_feed", feedViewData())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, `id="activity-feed"`) {
		t.Errorf("fragment missing its wrapper: %s", got)
	}
	for _, chrome := range []string{"<!doctype", "<html", "<body", "topbar"} {
		if strings.Contains(got, chrome) {
			t.Errorf("fragment contains layout markup %q: %s", chrome, got)
		}
	}
}

// A fragment is still a page response: it must carry the same no-store headers,
// or a swapped-in region could be replayed from the browser cache.
func TestRenderFragmentSetsNoStoreHeaders(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	rec := httptest.NewRecorder()
	r.RenderFragment(rec, http.StatusOK, "activity", "frag_activity_feed", feedViewData())

	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// An unknown page or block is a wiring mistake, so it fails loudly rather than
// returning 200 with an empty body that would silently blank out a region.
func TestRenderFragmentUnknownFailsLoudly(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	rec := httptest.NewRecorder()
	r.RenderFragment(rec, http.StatusOK, "nope", "frag_activity_feed", feedViewData())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unknown page status = %d, want 500", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.RenderFragment(rec, http.StatusOK, "activity", "frag_nope", feedViewData())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unknown block status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "frag_nope") {
		t.Errorf("error body should name the missing block: %s", rec.Body.String())
	}
}

// The fragment and the full page must render the same region, or a swap would
// quietly replace it with different markup.
func TestFragmentMatchesRegionInFullPage(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	frag := httptest.NewRecorder()
	r.RenderFragment(frag, http.StatusOK, "activity", "frag_activity_feed", feedViewData())
	full := httptest.NewRecorder()
	r.Render(full, http.StatusOK, "activity", feedViewData())

	region := strings.TrimSpace(frag.Body.String())
	if region == "" {
		t.Fatal("fragment rendered empty")
	}
	if !strings.Contains(full.Body.String(), region) {
		t.Errorf("full page does not contain the fragment verbatim:\n--- fragment ---\n%s", region)
	}
}
