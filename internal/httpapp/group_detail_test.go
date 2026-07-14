package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestGroupDetailPolish checks the refreshed group page: identity header with a
// member avatar stack, the overflow menu holding archive/simplify, and a settle
// button on a settlement the current user owes.
func TestGroupDetailPolish(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})

	// Bob(2) pays for everyone → Alice(1) owes Bob, so Alice sees a Settle button.
	h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})

	b := body(t, h.get("/groups/1"))
	for _, want := range []string{`class="ava-stack"`, "icons.svg?v=", "#dots", "#archive", "month-h"} {
		if !strings.Contains(b, want) {
			t.Errorf("group page missing %q", want)
		}
	}
	// Alice owes Bob → settle link to bob's friend settle route.
	if !strings.Contains(b, `/friends/2/settle`) {
		t.Errorf("expected a settle button linking to /friends/2/settle")
	}

	// Overflow archive form still works (posts and redirects).
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if resp := h.post("/groups/1/archive", url.Values{}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("archive status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
}
