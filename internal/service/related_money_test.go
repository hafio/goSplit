package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// TestDirectMoneyNeedsARelationship: outside a group, an expense, a settlement
// or a conversion involving someone the actor shares nothing with is refused
// with ErrNotRelated and writes nothing -- no expense, no friendship, no
// notification. Inside a group, a conversion gets the members check.
func TestDirectMoneyNeedsARelationship(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	stranger := covUser(t, svc, "S", "s@x.com")
	expenses, notes := countRows(t, svc, "expenses"), countRows(t, svc, "notifications")

	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: stranger.ID}},
	}
	if _, err := svc.AddExpense(ctx, in); !errors.Is(err, ErrNotRelated) {
		t.Errorf("direct expense with a stranger: got %v, want ErrNotRelated", err)
	}
	paidByStranger := in
	paidByStranger.PaidBy, paidByStranger.Lines = stranger.ID, []split.Line{{UserID: a.ID}}
	if _, err := svc.AddExpense(ctx, paidByStranger); !errors.Is(err, ErrNotRelated) {
		t.Errorf("direct expense paid by a stranger: got %v, want ErrNotRelated", err)
	}
	if _, err := svc.Settle(ctx, a.ID, stranger.ID, 500, "USD", nil, "2026-01-10", a.ID); !errors.Is(err, ErrNotRelated) {
		t.Errorf("settling with a stranger: got %v, want ErrNotRelated", err)
	}
	if _, err := svc.CreateConversionExact(ctx, a.ID, a.ID, stranger.ID, 100, 90, "USD", "EUR", "2026-01-10", nil); !errors.Is(err, ErrNotRelated) {
		t.Errorf("converting with a stranger: got %v, want ErrNotRelated", err)
	}
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Solo", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateConversionExact(ctx, a.ID, a.ID, stranger.ID, 100, 90, "USD", "EUR", "2026-01-10", &g.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("group conversion with a non-member: got %v, want ErrNotMember", err)
	}

	if n := countRows(t, svc, "expenses"); n != expenses {
		t.Errorf("expenses %d -> %d after refused writes", expenses, n)
	}
	if n := countRows(t, svc, "notifications"); n != notes {
		t.Errorf("notifications %d -> %d after refused writes", notes, n)
	}
	if ok, _ := svc.Store.AreFriends(ctx, a.ID, stranger.ID); ok {
		t.Error("a refused expense befriended the stranger")
	}
}

// TestMoveExpenseKeepsDirectSplitsAmongRelatedPeople: an edit cannot pull a
// stranger into a direct expense. An admin outside the expense is checked
// through its payer instead of themselves, so they can re-save it as it is but
// cannot add the stranger either.
func TestMoveExpenseKeepsDirectSplitsAmongRelatedPeople(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, b, stranger := covUser(t, svc, "A", "a@x.com"), covUser(t, svc, "B", "b@x.com"), covUser(t, svc, "S", "s@x.com")
	covFriends(t, svc, a, b)
	admin, err := svc.Store.CreateUser(ctx, &store.User{Name: "Root", Email: "root@x.com", Role: store.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	}
	e, err := svc.AddExpense(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	withStranger := in
	withStranger.Version = e.Version
	withStranger.Lines = append([]split.Line{{UserID: stranger.ID}}, in.Lines...)
	if _, err := svc.MoveExpense(ctx, e.ID, withStranger, false); !errors.Is(err, ErrNotRelated) {
		t.Errorf("member adding a stranger: got %v, want ErrNotRelated", err)
	}
	withStranger.ActorID = admin.ID
	if _, err := svc.MoveExpense(ctx, e.ID, withStranger, false); !errors.Is(err, ErrNotRelated) {
		t.Errorf("admin adding a stranger: got %v, want ErrNotRelated", err)
	}
	if got, _ := svc.Store.GetExpense(ctx, e.ID); got.Version != e.Version {
		t.Fatalf("a refused edit changed the expense (version %d -> %d)", e.Version, got.Version)
	}

	resave := in
	resave.ActorID, resave.Version = admin.ID, e.Version
	if _, err := svc.MoveExpense(ctx, e.ID, resave, false); err != nil {
		t.Errorf("admin re-saving a split among related people: %v", err)
	}
}
