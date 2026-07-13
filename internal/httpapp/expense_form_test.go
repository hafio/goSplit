package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestExpenseFormRendersComponents(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	b := body(t, h.get("/expenses/new"))
	for _, want := range []string{"amount-hero", `class="seg"`, `name="method"`, "participants-list",
		"expense_form.js", "target-bar", `name="group_id"`} {
		if !strings.Contains(b, want) {
			t.Errorf("add-expense form missing %q", want)
		}
	}
}

// TestExpenseFormTargetSelector checks the add form shows the target group/direct
// selector and preselects the group when opened in a group context.
func TestExpenseFormTargetSelector(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})

	// Direct context: the "Direct (no group)" option is selected.
	if b := body(t, h.get("/expenses/new")); !strings.Contains(b, `value="" selected`) {
		t.Errorf("direct add should preselect the no-group option")
	}
	// Group context: the group option is selected and its name shown.
	b := body(t, h.get("/expenses/new?group=1"))
	if !strings.Contains(b, `value="1" selected`) || !strings.Contains(b, "Trip") {
		t.Errorf("group add should preselect the group, got selector without it")
	}
}

// TestExpenseFormRoundTripsMethods confirms the redesigned form still posts the
// frozen field contract for a non-equal split method.
func TestExpenseFormRoundTripsMethods(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	// PERCENTAGE split, 50/50 between alice(1) and bob(2).
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	resp := h.post("/expenses", url.Values{
		"name": {"Lunch"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-03-09"}, "method": {"PERCENTAGE"}, "paid_by": {"1"},
		"category":  {"Food & drink"},
		"include_1": {"1"}, "value_1": {"50"},
		"include_2": {"1"}, "value_2": {"50"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("percentage expense post status %d", resp.StatusCode)
	}
	if !strings.Contains(body(t, resp), "Lunch") {
		t.Fatal("percentage expense detail missing name")
	}

	// The activity feed shows the category emoji for the stored category.
	if a := body(t, h.get("/activity")); !strings.Contains(a, "🍕") {
		t.Errorf("activity feed missing category emoji for Food & drink")
	}
}
