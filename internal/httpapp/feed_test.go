package httpapp

import "testing"

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
