package store

import (
	"context"
	"database/sql"
	"sort"
	"testing"
)

// nullGroup turns an optional group id into the column value.
func nullGroup(gid *int64) sql.NullInt64 {
	if gid == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *gid, Valid: true}
}

// amountPtr returns a pointer to a minor-unit amount, for ExpenseFilter bounds.
func amountPtr(v int64) *int64 { return &v }

// expenseNames returns the expense names in feed order.
func expenseNames(es []*Expense) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

// TestListAllActivity covers the admin feed query: with no user and nothing else
// to narrow it, it still builds a valid query (the 1 = 1 seed) and returns every
// row, soft-deleted ones included; a user means paid, created or participant --
// not merely a member of the group -- and stays correct combined with other
// filters (the parenthesized OR); a group filter and the limit apply as usual.
func TestListAllActivity(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a, b, c := covUser(t, st, "a"), covUser(t, st, "b"), covUser(t, st, "c")
	g, _ := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	_ = st.AddGroupMember(ctx, g.ID, b.ID)
	_ = st.AddGroupMember(ctx, g.ID, c.ID)

	mk := func(name, date string, payer, addedBy int64, gid *int64, parts []ExpenseParticipant) *Expense {
		t.Helper()
		var amt int64
		for _, p := range parts {
			if p.Amount > 0 {
				amt += p.Amount
			}
		}
		e, err := st.CreateExpense(ctx, &Expense{Name: name, Category: "general", Amount: amt, SplitType: "EQUAL",
			ExpenseDate: date, Currency: "USD", PaidBy: payer, AddedBy: addedBy, GroupID: nullGroup(gid)}, parts)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return e
	}
	// ab: a pays for a+b in G. bc: b pays for b+c in G but a added it. direct: a
	// and c, no group. gone: deleted later.
	mk("ab", "2026-01-01", a.ID, a.ID, &g.ID, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mk("bc", "2026-01-02", b.ID, a.ID, &g.ID, []ExpenseParticipant{{UserID: b.ID, Amount: 700}, {UserID: c.ID, Amount: -700}})
	mk("direct", "2026-01-03", c.ID, c.ID, nil, []ExpenseParticipant{{UserID: c.ID, Amount: 300}, {UserID: a.ID, Amount: -300}})
	gone := mk("gone", "2026-01-04", b.ID, b.ID, nil, []ExpenseParticipant{{UserID: b.ID, Amount: 100}, {UserID: c.ID, Amount: -100}})
	if err := st.SoftDeleteExpense(ctx, gone.ID, b.ID, gone.Version); err != nil {
		t.Fatal(err)
	}

	check := func(label string, userID int64, f ExpenseFilter, want ...string) {
		t.Helper()
		got, err := st.ListAllActivity(ctx, userID, f)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		names := expenseNames(got)
		sort.Strings(names)
		sort.Strings(want)
		if len(names) != len(want) {
			t.Fatalf("%s = %v, want %v", label, names, want)
		}
		for i := range want {
			if names[i] != want[i] {
				t.Fatalf("%s = %v, want %v", label, names, want)
			}
		}
	}
	check("everyone, archived included", 0, ExpenseFilter{IncludeArchived: true}, "ab", "bc", "direct", "gone")
	// a paid ab, created bc (without being in it) and is a participant of direct.
	check("user a", a.ID, ExpenseFilter{IncludeArchived: true}, "ab", "bc", "direct")
	// c is a member of G but not part of ab: membership alone does not count.
	check("user c", c.ID, ExpenseFilter{IncludeArchived: true}, "bc", "direct", "gone")
	check("user a, min 6.00", a.ID, ExpenseFilter{IncludeArchived: true, AmountMin: amountPtr(600)}, "bc")
	check("user a, search b", a.ID, ExpenseFilter{IncludeArchived: true, Descr: "b"}, "ab", "bc")
	check("user c, in G", c.ID, ExpenseFilter{GroupID: &g.ID}, "bc")
	check("group G", 0, ExpenseFilter{GroupID: &g.ID}, "ab", "bc")

	limited, err := st.ListAllActivity(ctx, 0, ExpenseFilter{IncludeArchived: true, Limit: 2})
	if err != nil || len(limited) != 2 || limited[0].Name != "gone" || limited[1].Name != "direct" {
		t.Errorf("limit 2 = %v (err %v), want the two newest [gone direct]", expenseNames(limited), err)
	}
}

// TestListAllGroupsAndCounts covers the admin group list: every group by name,
// archived ones only when asked, and member counts for all of them.
func TestListAllGroupsAndCounts(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a, b := covUser(t, st, "a"), covUser(t, st, "b")
	zed, _ := st.CreateGroup(ctx, &Group{Name: "Zed", CreatedBy: a.ID})
	alpha, _ := st.CreateGroup(ctx, &Group{Name: "Alpha", CreatedBy: b.ID})
	_ = st.AddGroupMember(ctx, zed.ID, b.ID)
	if err := st.SetGroupArchived(ctx, alpha.ID, true); err != nil {
		t.Fatal(err)
	}

	all, err := st.ListAllGroups(ctx, true)
	if err != nil || len(all) != 2 || all[0].Name != "Alpha" || all[1].Name != "Zed" {
		t.Fatalf("ListAllGroups(true) = %v (err %v), want [Alpha Zed]", all, err)
	}
	live, err := st.ListAllGroups(ctx, false)
	if err != nil || len(live) != 1 || live[0].ID != zed.ID {
		t.Fatalf("ListAllGroups(false) = %v (err %v), want only Zed", live, err)
	}
	counts, err := st.AllGroupMemberCounts(ctx)
	if err != nil || counts[zed.ID] != 2 || counts[alpha.ID] != 1 {
		t.Errorf("AllGroupMemberCounts = %v (err %v), want Zed 2, Alpha 1", counts, err)
	}
}
