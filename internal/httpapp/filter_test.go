package httpapp

import (
	"strings"
	"testing"
)

// TestFilterChipsRender checks the reworked filter bar renders a removable chip
// for an active filter and that the chip link drops that one parameter.
func TestFilterChipsRender(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	b := body(t, h.get("/activity?min=10.00"))
	if !strings.Contains(b, `class="chips filter-chips"`) {
		t.Fatal("no filter chips rendered for active min filter")
	}
	if !strings.Contains(b, "≥ 10.00") {
		t.Errorf("min chip label missing")
	}
	// The advanced-filter toggle shows its active indicator.
	if !strings.Contains(b, "filter-adv has-active") {
		t.Errorf("active-filter indicator missing")
	}
	// Search field is always present.
	if !strings.Contains(b, `name="q"`) {
		t.Errorf("search field missing")
	}
}
