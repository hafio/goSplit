package httpapp

import (
	"net/url"
	"strings"
	"testing"
)

// TestListsRenderComponents checks the refreshed lists render avatar rows and
// the active-nav marker rather than the old data tables.
func TestListsRenderComponents(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	friends := body(t, h.get("/friends"))
	if !strings.Contains(friends, `class="ava"`) {
		t.Errorf("friends page missing avatar rows")
	}
	if !strings.Contains(friends, "icons.svg#chev-right") {
		t.Errorf("friends page missing chevron icon")
	}

	balances := body(t, h.get("/balances"))
	// Active primary-nav item gets the "on" class on the current page.
	if !strings.Contains(balances, `href="/balances" class="on"`) {
		t.Errorf("balances page missing active nav marker")
	}
}
