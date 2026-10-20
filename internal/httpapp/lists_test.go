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
	if !strings.Contains(friends, "#chev-right") {
		t.Errorf("friends page missing chevron icon")
	}

	balances := body(t, h.get("/balances"))
	// Active primary-nav item gets the "on" class on the current page.
	if !strings.Contains(balances, `href="/balances" class="on"`) {
		t.Errorf("balances page missing active nav marker")
	}
}

// TestBalancesNoteOnlyWithBalances pins the "not simplified" warning on the
// Balances page: it sits above the per-person list once there is a balance, and
// links to the Groups page where the simplified debts live. Asserting the link
// text matters because the nav renders href="/groups" on every page anyway.
func TestBalancesNoteOnlyWithBalances(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	settled := body(t, h.get("/balances"))
	for _, unwanted := range []string{`class="flash warn"`, "not netted off or simplified"} {
		if strings.Contains(settled, unwanted) {
			t.Errorf("balances page with nothing owed shows the note (%q)", unwanted)
		}
	}

	h = tripHarness(t) // Alice owes Bob 50.00 inside group 1
	page := body(t, h.get("/balances"))
	for _, want := range []string{
		`class="flash warn"`,
		"These per-person totals are not netted off or simplified across people.",
		`<a href="/groups">For simplified debts, open a group from the Groups page.</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("balances page missing %q", want)
		}
	}
	if note, list := strings.Index(page, `class="flash warn"`), strings.Index(page, `class="l-row"`); note < 0 || list < 0 || note > list {
		t.Errorf("note must sit above the per-person list (note at %d, first row at %d)", note, list)
	}
}
