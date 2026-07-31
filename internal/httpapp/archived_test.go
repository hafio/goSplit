package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestArchivedGroupHiddenFromActivityAndSection verifies the end-to-end archived
// behavior: an archived group's expense leaves the default activity feed but
// returns under ?archived=1 (parseFilter -> IncludeArchived), and the groups
// page moves the group into its collapsed "Archived" section (handleGroups
// active/archived split + groups.html).
func TestArchivedGroupHiddenFromActivityAndSection(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"SkiTrip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}}) // Bob(2) becomes a member

	// A group expense involving Alice(1).
	h.post("/expenses", url.Values{
		"name": {"Chalet"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})

	// Before archiving the expense is in the default activity feed.
	if !strings.Contains(body(t, h.get("/activity")), "Chalet") {
		t.Fatal("pre-archive activity missing Chalet")
	}

	// Archive the group (POST redirects with 303).
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if resp := h.post("/groups/1/archive", url.Values{}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("archive status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
	h.client.CheckRedirect = nil

	// Default activity feed hides the archived group's expense...
	if strings.Contains(body(t, h.get("/activity")), "Chalet") {
		t.Error("archived-group expense should be hidden from /activity by default")
	}
	// ...but the activity page offers the Archived filter checkbox...
	if !strings.Contains(body(t, h.get("/activity")), `name="archived"`) {
		t.Error("/activity should offer the Archived filter checkbox")
	}
	// ...and opting in brings the expense back.
	if !strings.Contains(body(t, h.get("/activity?archived=1")), "Chalet") {
		t.Error("/activity?archived=1 should include the archived-group expense")
	}

	// The groups page renders a collapsed Archived section holding the group.
	gb := body(t, h.get("/groups"))
	if !strings.Contains(gb, "arch-section") {
		t.Error("/groups should render the collapsed archived section")
	}
	if !strings.Contains(gb, "SkiTrip") {
		t.Error("/groups archived section should list the archived group")
	}
}
