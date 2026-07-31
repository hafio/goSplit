package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

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

	newExpense := func() string {
		e, err := svc.Store.CreateExpense(ctx, &store.Expense{
			Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL", ExpenseDate: "2026-01-10",
			Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullInt(&g.ID),
		}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
		if err != nil {
			t.Fatalf("create expense: %v", err)
		}
		return e.ID
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
		id := newExpense()
		if err := svc.DeleteExpense(ctx, id, actor); !errors.Is(err, ErrNotEditor) {
			t.Errorf("delete by non-editor %d: got %v, want ErrNotEditor", actor, err)
		}
		var deleted int
		_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE id = ? AND deleted_at IS NOT NULL`, id).Scan(&deleted)
		if deleted != 0 {
			t.Errorf("expense %s soft-deleted by non-editor %d", id, actor)
		}
	}

	// A missing id reports not-found.
	if err := svc.DeleteExpense(ctx, "no-such-id", a.ID); !errors.Is(err, ErrExpenseNotFound) {
		t.Errorf("delete missing id: got %v, want ErrExpenseNotFound", err)
	}

	// The participant and the creator can each delete; a successful delete restores
	// the balance exactly (measured as a delta so earlier live rows don't matter).
	for _, actor := range []int64{b.ID, a.ID} {
		before := pairBalance()
		id := newExpense()
		if err := svc.DeleteExpense(ctx, id, actor); err != nil {
			t.Fatalf("delete by member %d: %v", actor, err)
		}
		var deletedAt sql.NullString
		_ = svc.Store.DB.QueryRow(`SELECT deleted_at FROM expenses WHERE id = ?`, id).Scan(&deletedAt)
		if !deletedAt.Valid {
			t.Errorf("expense %s not soft-deleted after a member delete", id)
		}
		if after := pairBalance(); after != before {
			t.Errorf("balance not restored after deleting %s: before=%d after=%d", id, before, after)
		}
	}
}
