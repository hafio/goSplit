package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestExpenseNoteAndCollapse(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	// Direct expense between alice(1) and bob(2).
	resp := h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-01-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	path := resp.Request.URL.Path // /expenses/<id> after the redirect
	_ = body(t, resp)
	if !strings.HasPrefix(path, "/expenses/") {
		t.Fatalf("no expense id in redirect path %q", path)
	}

	// Note round-trip via the edit form (same group, so no ack needed).
	h.post(path+"/move", url.Values{
		"target_group": {"none"}, "name": {"Dinner"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-01-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"}, "note": {"Split the tasting menu"},
	})
	if !strings.Contains(body(t, h.get(path)), "Split the tasting menu") {
		t.Errorf("note not shown on expense detail after edit")
	}

	// A second older direct expense, then collapse everything before a cutoff.
	h.post("/expenses", url.Values{
		"name": {"Taxi"}, "amount": {"10.00"}, "currency": {"USD"},
		"date": {"2026-01-05"}, "method": {"EQUAL"}, "paid_by": {"2"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	if r := h.post("/friends/2/collapse", url.Values{"before": {"2026-03-01"}}); r.StatusCode != http.StatusOK {
		t.Fatalf("collapse status %d", r.StatusCode)
	} else {
		_ = body(t, r)
	}

	feed := body(t, h.get("/friends/2"))
	if !strings.Contains(feed, "Historical Transactions") {
		t.Errorf("friend feed missing Historical Transactions after collapse")
	}
	if strings.Contains(feed, ">Dinner<") || strings.Contains(feed, ">Taxi<") {
		t.Errorf("collapsed originals still shown in the feed")
	}
}
