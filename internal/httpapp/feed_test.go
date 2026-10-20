package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

func TestGroupByMonth(t *testing.T) {
	rows := []expenseRow{
		{ID: "1", Date: "2026-03-08"},
		{ID: "2", Date: "2026-03-01"},
		{ID: "3", Date: "2026-02-20"},
	}
	groups := groupByMonth(rows)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if groups[0].Label != "March 2026" || len(groups[0].Rows) != 2 {
		t.Errorf("first group = %q with %d rows", groups[0].Label, len(groups[0].Rows))
	}
	if groups[1].Label != "February 2026" || len(groups[1].Rows) != 1 {
		t.Errorf("second group = %q with %d rows", groups[1].Label, len(groups[1].Rows))
	}
}

// longFeedHarness signs Alice in with one expense more than a feed page holds,
// so her activity feed is truncated and offers "Show all".
func longFeedHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	ctx := context.Background()

	alice, err := h.st.GetUserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < feedLimit+1; i++ {
		e := &store.Expense{
			Name: "Expense", Category: "general", Amount: 1000, SplitType: "EQUAL",
			ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: alice.ID, AddedBy: alice.ID,
		}
		parts := []store.ExpenseParticipant{{UserID: alice.ID, Amount: 1000}}
		if _, err := h.st.CreateExpense(ctx, e, parts); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func TestActivityFeedShowAll(t *testing.T) {
	h := longFeedHarness(t)

	limited := body(t, h.get("/activity"))
	if !strings.Contains(limited, "feed.show_all") && !strings.Contains(limited, "Show all") {
		t.Fatal("limited activity page missing show-all link")
	}

	all := body(t, h.get("/activity?"+url.Values{"all": {"1"}}.Encode()))
	if strings.Contains(all, "Show all") {
		t.Fatal("?all=1 page should not show the show-all link")
	}
}

// TestTruncatedFeedPollReturns204: the poller sends its last fingerprint as ?v=,
// and the "Show all" link must not carry it along. If it did, every render of a
// feed over the limit would differ from the one before, and the poll could never
// be answered 204 -- the whole page would be re-sent every ten seconds.
func TestTruncatedFeedPollReturns204(t *testing.T) {
	h := longFeedHarness(t)

	first := h.getHX("/activity?v=", "content")
	v := first.Header.Get("X-Fragment-Version")
	if b := body(t, first); !strings.Contains(b, `href="/activity?all=1"`) {
		t.Fatalf("the truncated feed's show-all link is not the bare all=1 link:\n%s", b)
	}
	again := h.getHX("/activity?v="+v, "content")
	_ = body(t, again)
	if again.StatusCode != http.StatusNoContent {
		t.Errorf("an unchanged truncated feed polled %d, want 204", again.StatusCode)
	}
}

func TestFeedDateParts(t *testing.T) {
	day, mon := feedDateParts("2026-03-08")
	if day != "08" || mon != "MAR" {
		t.Errorf("feedDateParts = %q %q, want 08 MAR", day, mon)
	}
	// Non-date input degrades without panicking.
	if day, mon := feedDateParts("garbage"); day != "garbage" || mon != "" {
		t.Errorf("feedDateParts(garbage) = %q %q", day, mon)
	}
}
