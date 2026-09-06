package web

import (
	"html/template"
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
	r.RenderFragment(rec, nil, http.StatusOK, "activity", "frag_activity_feed", feedViewData())
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
	r.RenderFragment(rec, nil, http.StatusOK, "activity", "frag_activity_feed", feedViewData())

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
	r.RenderFragment(rec, nil, http.StatusOK, "nope", "frag_activity_feed", feedViewData())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unknown page status = %d, want 500", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.RenderFragment(rec, nil, http.StatusOK, "activity", "frag_nope", feedViewData())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unknown block status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "frag_nope") {
		t.Errorf("error body should name the missing block: %s", rec.Body.String())
	}
}

// The freshness poller depends on this exchange: every fragment response
// carries a fingerprint, and a request echoing the one it already holds gets
// 204 with no body -- that is what makes a 10s poll nearly free.
func TestRenderFragmentVersionAnd204(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	first := httptest.NewRecorder()
	r.RenderFragment(first, httptest.NewRequest(http.MethodGet, "/activity", nil),
		http.StatusOK, "activity", "frag_activity_feed", feedViewData())
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}
	v := first.Header().Get(FragmentVersionHeader)
	if v == "" {
		t.Fatal("no fragment version header on the first render")
	}
	if first.Body.Len() == 0 {
		t.Fatal("first render returned no body")
	}

	// Same data, echoing the version back: unchanged, so no body.
	same := httptest.NewRecorder()
	r.RenderFragment(same, httptest.NewRequest(http.MethodGet, "/activity?v="+v, nil),
		http.StatusOK, "activity", "frag_activity_feed", feedViewData())
	if same.Code != http.StatusNoContent {
		t.Errorf("unchanged fragment status = %d, want 204", same.Code)
	}
	if same.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", same.Body.String())
	}
	if same.Header().Get(FragmentVersionHeader) != v {
		t.Error("204 must still carry the version header")
	}

	// A stale version means the client is behind: send the body.
	stale := httptest.NewRecorder()
	r.RenderFragment(stale, httptest.NewRequest(http.MethodGet, "/activity?v=deadbeef", nil),
		http.StatusOK, "activity", "frag_activity_feed", feedViewData())
	if stale.Code != http.StatusOK || stale.Body.Len() == 0 {
		t.Errorf("stale version got %d with %d bytes, want 200 with a body", stale.Code, stale.Body.Len())
	}
}

// Different content must fingerprint differently, or a change would be
// swallowed as a 204 and the page would sit stale forever.
func TestFragmentVersionTracksContent(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	version := func(vd ViewData) string {
		rec := httptest.NewRecorder()
		r.RenderFragment(rec, nil, http.StatusOK, "activity", "frag_activity_feed", vd)
		return rec.Header().Get(FragmentVersionHeader)
	}

	empty := feedViewData()
	filtered := feedViewData()
	filtered.Data.(map[string]any)["Filter"] = map[string]string{"q": "dinner"}
	if version(empty) == version(filtered) {
		t.Error("a changed filter produced the same fingerprint")
	}
}

// The registry is the single place a page declares a swappable region, so a
// typo there must stop the process at startup, not 500 on the first swap.
func TestFragmentRegistryIsValid(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	for page, byTarget := range fragments {
		t.Run(page, func(t *testing.T) {
			tmpl, ok := r.pages[page]
			if !ok {
				t.Fatalf("registry names unknown page %q", page)
			}
			for target, f := range byTarget {
				if tmpl.Lookup(f.Block) == nil {
					t.Errorf("target %q names unknown block %q", target, f.Block)
				}
			}
		})
	}

	// And the validator itself rejects both kinds of bad entry, so a typo can
	// never reach production as a runtime 500.
	pages := map[string]*template.Template{"activity": r.pages["activity"]}
	if err := validateFragments(pages, map[string]map[string]Fragment{
		"no_such_page": {"content": {Block: "content"}},
	}); err == nil {
		t.Error("validateFragments accepted a page that does not exist")
	}
	if err := validateFragments(pages, map[string]map[string]Fragment{
		"activity": {"content": {Block: "no_such_block"}},
	}); err == nil {
		t.Error("validateFragments accepted a block that does not exist")
	}
}

// FragmentFor is what the server consults on every render to decide whether an
// htmx request names a swappable region.
func TestFragmentForLookup(t *testing.T) {
	if f, ok := FragmentFor("group", "group-feed"); !ok || f.Block != "frag_group_feed" {
		t.Errorf("FragmentFor(group, group-feed) = %+v, %v", f, ok)
	}
	if f, ok := FragmentFor("group", "content"); !ok || f.Block != "content" || !f.Poll {
		t.Errorf("FragmentFor(group, content) = %+v, %v", f, ok)
	}
	// A boosted navigation targets the body, and a plain one names nothing --
	// neither may resolve, or every page load would answer with a bare fragment.
	for _, target := range []string{"body", "", "made-up"} {
		if _, ok := FragmentFor("group", target); ok {
			t.Errorf("FragmentFor(group, %q) resolved; it must not", target)
		}
	}
	if _, ok := FragmentFor("no_such_page", "content"); ok {
		t.Error("FragmentFor resolved an unknown page")
	}
}

// A block whose data will not resolve must fail loudly. Silently emitting a
// half-written region is the bug that made a mistyped password return a
// truncated profile page.
func TestRenderFragmentReportsTemplateFailure(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	rec := httptest.NewRecorder()
	// No Data at all, but the feed block dereferences .Data.Items.
	r.RenderFragment(rec, nil, http.StatusOK, "activity", "frag_activity_feed", ViewData{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the block cannot render", rec.Code)
	}
	if rec.Header().Get(FragmentVersionHeader) != "" {
		t.Error("a failed render must not publish a fingerprint")
	}
}

// Only list and detail pages may be polled; a form page must never be, or a
// timed morph would discard what the user was typing.
func TestPollableCoversListPagesOnly(t *testing.T) {
	for _, page := range []string{"balances", "friends", "friend", "groups", "group", "activity", "recurring", "expense_detail"} {
		if !pollable(page) {
			t.Errorf("%s should be pollable", page)
		}
	}
	for _, page := range []string{"expense_form", "convert", "settle_group", "collapse", "profile", "admin", "import", "bank", "login", "register", ""} {
		if pollable(page) {
			t.Errorf("%s must not be pollable", page)
		}
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
	r.RenderFragment(frag, nil, http.StatusOK, "activity", "frag_activity_feed", feedViewData())
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
