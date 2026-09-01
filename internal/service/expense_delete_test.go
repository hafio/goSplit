package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// TestDeleteExpenseAuthorization pins the delete rule: only a member of the
// transaction (payer/creator/participant) may delete it. A group member who is not
// a participant and an outright outsider are both refused; a missing id reports
// ErrExpenseNotFound; and a successful delete leaves the row soft-deleted with its
// balance restored (the row drops out of balance_view). Deleting is the supported
// way to reverse a settlement.
func TestDeleteExpenseAuthorization(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"}) // creator + payer
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"}) // participant
	c, _ := svc.Store.CreateUser(ctx, &store.User{Name: "C", Email: "c@x.com"}) // group member, not a participant
	d, _ := svc.Store.CreateUser(ctx, &store.User{Name: "D", Email: "d@x.com"}) // outsider
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	for _, id := range []int64{a.ID, b.ID, c.ID} {
		_ = svc.Store.AddGroupMember(ctx, g.ID, id)
	}

	newExpense := func() *store.Expense {
		e, err := svc.Store.CreateExpense(ctx, &store.Expense{
			Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL", ExpenseDate: "2026-01-10",
			Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullInt(&g.ID),
		}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
		if err != nil {
			t.Fatalf("create expense: %v", err)
		}
		return e
	}
	pairBalance := func() int64 {
		var bal int64
		_ = svc.Store.DB.QueryRow(
			`SELECT COALESCE(SUM(amount), 0) FROM balance_view WHERE group_id = ? AND user_id = ? AND friend_id = ?`,
			g.ID, a.ID, b.ID).Scan(&bal)
		return bal
	}

	// Non-editors are refused and nothing is deleted: a group member who is not a
	// participant, and an outsider.
	for _, actor := range []int64{c.ID, d.ID} {
		e := newExpense()
		if err := svc.DeleteExpense(ctx, e.ID, actor, e.Version); !errors.Is(err, ErrNotEditor) {
			t.Errorf("delete by non-editor %d: got %v, want ErrNotEditor", actor, err)
		}
		var deleted int
		_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE id = ? AND deleted_at IS NOT NULL`, e.ID).Scan(&deleted)
		if deleted != 0 {
			t.Errorf("expense %s soft-deleted by non-editor %d", e.ID, actor)
		}
	}

	// A missing id reports not-found.
	if err := svc.DeleteExpense(ctx, "no-such-id", a.ID, store.InitialVersion); !errors.Is(err, ErrExpenseNotFound) {
		t.Errorf("delete missing id: got %v, want ErrExpenseNotFound", err)
	}

	// The participant and the creator can each delete; a successful delete restores
	// the balance exactly (measured as a delta so earlier live rows don't matter).
	for _, actor := range []int64{b.ID, a.ID} {
		before := pairBalance()
		e := newExpense()
		if err := svc.DeleteExpense(ctx, e.ID, actor, e.Version); err != nil {
			t.Fatalf("delete by member %d: %v", actor, err)
		}
		var deletedAt sql.NullString
		_ = svc.Store.DB.QueryRow(`SELECT deleted_at FROM expenses WHERE id = ?`, e.ID).Scan(&deletedAt)
		if !deletedAt.Valid {
			t.Errorf("expense %s not soft-deleted after a member delete", e.ID)
		}
		if after := pairBalance(); after != before {
			t.Errorf("balance not restored after deleting %s: before=%d after=%d", e.ID, before, after)
		}
	}
}

// TestDeleteExpenseRejectsStaleVersion: the delete button carries the version the
// detail page was rendered from, so a delete decided before someone else's edit
// landed is refused instead of discarding that edit unseen.
func TestDeleteExpenseRejectsStaleVersion(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	}
	e, err := svc.AddExpense(ctx, in)
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	// B opens the detail page (version e.Version); A edits it meanwhile.
	edit := in
	edit.Name = "Brunch"
	edit.Version = e.Version
	if _, err := svc.MoveExpense(ctx, e.ID, edit, false); err != nil {
		t.Fatalf("competing edit: %v", err)
	}
	if err := svc.DeleteExpense(ctx, e.ID, b.ID, e.Version); !errors.Is(err, store.ErrStaleExpense) {
		t.Fatalf("stale delete: err = %v, want ErrStaleExpense", err)
	}
	live, _ := svc.Store.GetExpense(ctx, e.ID)
	if live.DeletedAt.Valid {
		t.Error("a stale delete removed the expense anyway")
	}
	// Reloading the page hands back the current version, and the delete works.
	if err := svc.DeleteExpense(ctx, e.ID, b.ID, live.Version); err != nil {
		t.Fatalf("delete at the current version: %v", err)
	}
}
