package store

import (
	"context"
	"database/sql"
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

func TestExpenseFilterLimit(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	a, _ := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	b, _ := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com"})

	dates := []string{"2025-01-01", "2025-01-02", "2025-01-03", "2025-01-04", "2025-01-05"}
	for _, d := range dates {
		e := &Expense{
			Name: "Expense " + d, Category: "general", Amount: 1000, SplitType: "EQUAL",
			ExpenseDate: d, Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
		}
		parts := []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}}
		if _, err := st.CreateExpense(ctx, e, parts); err != nil {
			t.Fatal(err)
		}
	}

	es, err := st.ListFriendExpenses(ctx, a.ID, b.ID, ExpenseFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("limit 2: got %d expenses", len(es))
	}
	if es[0].ExpenseDate != "2025-01-05" || es[1].ExpenseDate != "2025-01-04" {
		t.Fatalf("limit did not keep newest-first order: %+v", es)
	}

	all, err := st.ListFriendExpenses(ctx, a.ID, b.ID, ExpenseFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(dates) {
		t.Fatalf("no limit: got %d want %d", len(all), len(dates))
	}
}

func TestGroupMemberCountsAndNets(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	a, _ := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	b, _ := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com"})
	c, _ := st.CreateUser(ctx, &User{Name: "C", Email: "c@x.com"})

	g1, err := st.CreateGroup(ctx, &Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g1.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	g2, err := st.CreateGroup(ctx, &Group{Name: "House", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g2.ID, c.ID); err != nil {
		t.Fatal(err)
	}

	counts, err := st.GroupMemberCounts(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if counts[g1.ID] != 2 || counts[g2.ID] != 2 {
		t.Fatalf("member counts = %+v, want 2 for both groups", counts)
	}

	e := &Expense{
		Name: "Groceries", Category: "general", Amount: 4000, SplitType: "EQUAL",
		ExpenseDate: "2025-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: sql.NullInt64{Int64: g1.ID, Valid: true},
	}
	parts := []ExpenseParticipant{{UserID: a.ID, Amount: 2000}, {UserID: b.ID, Amount: -2000}}
	if _, err := st.CreateExpense(ctx, e, parts); err != nil {
		t.Fatal(err)
	}

	nets, err := st.UserGroupNets(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if nets[g1.ID]["USD"] != 2000 {
		t.Fatalf("UserGroupNets[g1][USD] = %d, want 2000", nets[g1.ID]["USD"])
	}
	bals, err := st.UserGroupBalances(ctx, g1.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var want int64
	for _, bal := range bals {
		want += bal.Amount
	}
	if nets[g1.ID]["USD"] != want {
		t.Fatalf("UserGroupNets disagrees with UserGroupBalances: %d vs %d", nets[g1.ID]["USD"], want)
	}
}
