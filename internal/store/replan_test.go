package store

import (
	"context"
	"testing"
)

// TestReplanGroup covers the netting writer's store half: a missing group is a
// silent no-op (the plan is not even consulted), the plan sees the group's
// balance rows, its expenses are written in the same transaction, and a set
// that fails part-way is not committed at all.
func TestReplanGroup(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a, b := covUser(t, st, "a"), covUser(t, st, "b")
	g, _ := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	_ = st.AddGroupMember(ctx, g.ID, b.ID)
	gid := g.ID
	if _, err := st.CreateExpense(ctx, &Expense{Name: "x", Category: "general", Amount: 100, SplitType: "EQUAL",
		ExpenseDate: "2026-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullGroup(&gid)},
		[]ExpenseParticipant{{UserID: a.ID, Amount: 50}, {UserID: b.ID, Amount: -50}}); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		var n int
		_ = st.DB.QueryRow(`SELECT COUNT(*) FROM expenses`).Scan(&n)
		return n
	}

	called := false
	if n, err := st.ReplanGroup(ctx, 9999, func([]Balance) []PlannedExpense { called = true; return nil }); n != 0 || err != nil || called {
		t.Fatalf("missing group: n=%d err=%v called=%v, want 0, nil, plan not called", n, err, called)
	}

	entry := func(name string, rows ...ExpenseParticipant) PlannedExpense {
		return PlannedExpense{Expense: &Expense{Name: name, Category: "netting", Amount: 50, SplitType: "NETTING",
			ExpenseDate: "2026-01-02", Currency: "USD", PaidBy: rows[0].UserID, AddedBy: a.ID, GroupID: nullGroup(&gid)}, Parts: rows}
	}
	var seen []Balance
	n, err := st.ReplanGroup(ctx, g.ID, func(rows []Balance) []PlannedExpense {
		seen = rows
		return []PlannedExpense{entry("n1", ExpenseParticipant{UserID: b.ID, Amount: 50}, ExpenseParticipant{UserID: a.ID, Amount: -50})}
	})
	if err != nil || n != 1 {
		t.Fatalf("ReplanGroup = %d, %v; want 1 entry written", n, err)
	}
	if len(seen) != 2 { // both directions of the one pair
		t.Errorf("plan saw %d balance rows, want 2", len(seen))
	}
	if got, _ := st.GroupBalances(ctx, g.ID); len(got) != 0 {
		t.Errorf("the entry should have cleared the pair, balances = %v", got)
	}

	before := count()
	if _, err := st.ReplanGroup(ctx, g.ID, func([]Balance) []PlannedExpense {
		return []PlannedExpense{
			entry("ok", ExpenseParticipant{UserID: a.ID, Amount: 50}, ExpenseParticipant{UserID: b.ID, Amount: -50}),
			// The same user twice violates the participants key: the whole set must roll back.
			entry("bad", ExpenseParticipant{UserID: a.ID, Amount: 50}, ExpenseParticipant{UserID: a.ID, Amount: -50}),
		}
	}); err == nil {
		t.Fatal("a failing entry should fail the whole replan")
	}
	if after := count(); after != before {
		t.Errorf("expenses %d -> %d: a failed replan committed part of its set", before, after)
	}
}
