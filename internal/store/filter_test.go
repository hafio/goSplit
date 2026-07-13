package store

import (
	"context"
	"testing"
)

func TestBuildLikePattern(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"dinner", "%dinner%"},
		{"Din*", "Din%"},
		{"*arty", "%arty"},
		{"a*b", "a%b"},
		{"50%", "%50\\%%"},   // literal % escaped, wrapped
		{"a_b", "%a\\_b%"},   // literal _ escaped
	}
	for _, c := range cases {
		if got := BuildLikePattern(c.in); got != c.want {
			t.Errorf("BuildLikePattern(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestListFriendExpensesFilters(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	a, _ := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	b, _ := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com"})

	mk := func(name string, amount int64, date string) {
		e := &Expense{
			Name: name, Category: "general", Amount: amount, SplitType: "EQUAL",
			ExpenseDate: date, Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
		}
		parts := []ExpenseParticipant{{UserID: a.ID, Amount: amount / 2}, {UserID: b.ID, Amount: -amount / 2}}
		if _, err := st.CreateExpense(ctx, e, parts); err != nil {
			t.Fatal(err)
		}
	}
	mk("Dinner Party", 4000, "2025-01-10")
	mk("Groceries", 2000, "2025-02-15")
	mk("Movie night", 3000, "2025-03-20")

	count := func(f ExpenseFilter) int {
		es, err := st.ListFriendExpenses(ctx, a.ID, b.ID, f)
		if err != nil {
			t.Fatal(err)
		}
		return len(es)
	}

	if n := count(ExpenseFilter{}); n != 3 {
		t.Fatalf("no filter: %d want 3", n)
	}
	// Case-insensitive substring.
	if n := count(ExpenseFilter{Descr: "dinner"}); n != 1 {
		t.Fatalf("descr dinner: %d want 1", n)
	}
	// Wildcard.
	if n := count(ExpenseFilter{Descr: "*night"}); n != 1 {
		t.Fatalf("descr *night: %d want 1", n)
	}
	// Amount range.
	min := int64(2500)
	if n := count(ExpenseFilter{AmountMin: &min}); n != 2 {
		t.Fatalf("amount>=2500: %d want 2", n)
	}
	// Date range (inclusive).
	if n := count(ExpenseFilter{DateFrom: "2025-02-01", DateTo: "2025-02-28"}); n != 1 {
		t.Fatalf("feb only: %d want 1", n)
	}
	// Combined AND.
	if n := count(ExpenseFilter{Descr: "grocer", DateFrom: "2025-01-01"}); n != 1 {
		t.Fatalf("combined: %d want 1", n)
	}
}
