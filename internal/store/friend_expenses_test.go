package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestListFriendExpensesIncludesGroups verifies the friend view returns every
// expense that affects the pair's balance — group expenses included — and
// excludes expenses that don't (a third party paid, or the friend isn't on it).
func TestListFriendExpensesIncludesGroups(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	a, _ := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	b, _ := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com"})
	c, _ := st.CreateUser(ctx, &User{Name: "C", Email: "c@x.com"})
	g, _ := st.CreateGroup(ctx, &Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})

	mk := func(name string, gid *int64, payer int64, parts []ExpenseParticipant) {
		e := &Expense{
			Name: name, Category: "general", Amount: 1000, SplitType: "EQUAL",
			ExpenseDate: "2026-03-01", Currency: "USD", PaidBy: payer, AddedBy: payer,
		}
		if gid != nil {
			e.GroupID = sql.NullInt64{Int64: *gid, Valid: true}
		}
		if _, err := st.CreateExpense(ctx, e, parts); err != nil {
			t.Fatal(err)
		}
	}

	// Affects a<->b:
	mk("Group dinner", &g.ID, a.ID, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mk("Direct taxi", nil, b.ID, []ExpenseParticipant{{UserID: b.ID, Amount: 500}, {UserID: a.ID, Amount: -500}})
	// Does NOT affect a<->b:
	mk("A & C only", &g.ID, a.ID, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: c.ID, Amount: -500}})
	mk("C paid a+b", &g.ID, c.ID, []ExpenseParticipant{{UserID: c.ID, Amount: 800}, {UserID: a.ID, Amount: -400}, {UserID: b.ID, Amount: -400}})

	got, err := st.ListFriendExpenses(ctx, a.ID, b.ID, ExpenseFilter{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range got {
		names[e.Name] = true
	}
	if !names["Group dinner"] || !names["Direct taxi"] {
		t.Errorf("friend view missing pair-affecting expenses: %v", names)
	}
	if names["A & C only"] || names["C paid a+b"] {
		t.Errorf("friend view included expenses that don't affect the pair: %v", names)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 friend expenses, got %d", len(got))
	}
}

func TestAdminUpdateUser(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	u, _ := st.CreateUser(ctx, &User{Name: "Old", Email: "old@x.com", Role: "USER"})

	u.Name, u.Email, u.Role, u.Currency, u.PreferredLanguage = "New", "new@x.com", "ADMIN", "EUR", "es"
	if err := st.AdminUpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "New" || got.Email != "new@x.com" || got.Role != "ADMIN" ||
		got.Currency != "EUR" || got.PreferredLanguage != "es" {
		t.Errorf("admin update not persisted: %+v", got)
	}
}
