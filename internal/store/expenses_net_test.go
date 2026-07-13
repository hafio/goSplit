package store

import (
	"context"
	"testing"
)

func TestUserNetByExpense(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	a, _ := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	b, _ := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com"})
	c, _ := st.CreateUser(ctx, &User{Name: "C", Email: "c@x.com"})

	e := &Expense{
		Name: "Dinner", Category: "food", Amount: 4000, SplitType: "EQUAL",
		ExpenseDate: "2026-03-08", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}
	parts := []ExpenseParticipant{{UserID: a.ID, Amount: 2000}, {UserID: b.ID, Amount: -2000}}
	created, err := st.CreateExpense(ctx, e, parts)
	if err != nil {
		t.Fatal(err)
	}

	// Payer lent, other borrowed, third party absent.
	if m, _ := st.UserNetByExpense(ctx, a.ID, []string{created.ID}); m[created.ID] != 2000 {
		t.Errorf("lender net = %d, want 2000", m[created.ID])
	}
	if m, _ := st.UserNetByExpense(ctx, b.ID, []string{created.ID}); m[created.ID] != -2000 {
		t.Errorf("borrower net = %d, want -2000", m[created.ID])
	}
	m, _ := st.UserNetByExpense(ctx, c.ID, []string{created.ID})
	if _, ok := m[created.ID]; ok {
		t.Errorf("non-participant should be absent, got %v", m)
	}

	// Empty id list is a no-op, not an error.
	if m, err := st.UserNetByExpense(ctx, a.ID, nil); err != nil || len(m) != 0 {
		t.Errorf("empty ids: got %v err %v", m, err)
	}
}
