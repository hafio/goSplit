package service

import (
	"context"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

func TestMoveExpenseEditsInPlace(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	gA, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "A", CreatedBy: a.ID, DefaultCurrency: "USD"})
	gB, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "B", CreatedBy: a.ID, DefaultCurrency: "USD"})
	for _, gid := range []int64{gA.ID, gB.ID} {
		_ = svc.Store.AddGroupMember(ctx, gid, a.ID)
		_ = svc.Store.AddGroupMember(ctx, gid, b.ID)
	}
	orig, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullInt(&gA.ID),
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}

	in := ExpenseInput{
		Name: "Dinner", Category: "general", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, GroupID: &gB.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}}, ActorID: a.ID,
		Version: orig.Version,
	}

	// Acknowledgment is required.
	if _, err := svc.MoveExpense(ctx, orig.ID, in, false); err != ErrMoveNoAck {
		t.Fatalf("no-ack: got %v, want ErrMoveNoAck", err)
	}

	moved, err := svc.MoveExpense(ctx, orig.ID, in, true)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID != orig.ID {
		t.Errorf("id changed: %s != %s (should edit in place)", moved.ID, orig.ID)
	}
	if moved.DeletedAt.Valid {
		t.Errorf("moved expense should not be marked deleted")
	}
	if !moved.GroupID.Valid || moved.GroupID.Int64 != gB.ID {
		t.Errorf("group not relocated to B: %+v", moved.GroupID)
	}
	if moved.CreatedAt != orig.CreatedAt {
		t.Errorf("created_at changed: %q != %q", moved.CreatedAt, orig.CreatedAt)
	}

	// Exactly one expense row — no duplicate, no soft-deleted ghost.
	var total, deleted int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses`).Scan(&total)
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE deleted_at IS NOT NULL`).Scan(&deleted)
	if total != 1 || deleted != 0 {
		t.Errorf("expected 1 live expense, got total=%d deleted=%d", total, deleted)
	}

	// Balance moved from group A to group B.
	var inA, inB int64
	_ = svc.Store.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM balance_view WHERE group_id=? AND user_id=? AND friend_id=?`, gA.ID, a.ID, b.ID).Scan(&inA)
	_ = svc.Store.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM balance_view WHERE group_id=? AND user_id=? AND friend_id=?`, gB.ID, a.ID, b.ID).Scan(&inB)
	if inA != 0 {
		t.Errorf("group A balance = %d, want 0 (moved out)", inA)
	}
	if inB != 500 {
		t.Errorf("group B balance = %d, want 500", inB)
	}
}

// TestEditSameGroupNoAck confirms a plain in-place edit (same group, no
// relocation) succeeds without the recalculated-split acknowledgment and updates
// the record's scalar fields.
func TestEditSameGroupNoAck(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	_ = svc.Store.AddGroupMember(ctx, g.ID, a.ID)
	_ = svc.Store.AddGroupMember(ctx, g.ID, b.ID)
	orig, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullInt(&g.ID),
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}

	// Same group, no ack, changed name + amount + note → succeeds and edits in place.
	in := ExpenseInput{
		Name: "Lunch", Category: "general", Total: 2000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-11", PaidBy: a.ID, GroupID: &g.ID, Note: "brunch spot",
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}}, ActorID: a.ID,
		Version: orig.Version,
	}
	got, err := svc.MoveExpense(ctx, orig.ID, in, false)
	if err != nil {
		t.Fatalf("same-group edit without ack should succeed, got %v", err)
	}
	if got.ID != orig.ID || got.Name != "Lunch" || got.Amount != 2000 || got.Note != "brunch spot" {
		t.Errorf("edit not applied in place: %+v", got)
	}
	var total int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses`).Scan(&total)
	if total != 1 {
		t.Errorf("expected 1 expense row, got %d", total)
	}
}

// TestMoveExpenseUnauthorized confirms a user who is not the payer/creator or a
// participant cannot edit (the unified move/edit path). Group membership alone is
// not enough — see TestMoveExpenseGroupMemberNotParticipant.
func TestMoveExpenseUnauthorized(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	c, _ := svc.Store.CreateUser(ctx, &store.User{Name: "C", Email: "c@x.com"})
	orig, _ := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL", ExpenseDate: "2026-01-10",
		Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})

	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD", ExpenseDate: "2026-01-10",
		PaidBy: a.ID, Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}}, ActorID: c.ID,
	}
	if _, err := svc.MoveExpense(ctx, orig.ID, in, true); err != ErrNotEditor {
		t.Fatalf("outsider edit: got %v, want ErrNotEditor", err)
	}
}

// TestMoveExpenseGroupMemberNotParticipant proves the tightened rule: a member of
// the expense's group who is not part of the transaction (not payer, creator, or a
// participant) still cannot edit it.
func TestMoveExpenseGroupMemberNotParticipant(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	c, _ := svc.Store.CreateUser(ctx, &store.User{Name: "C", Email: "c@x.com"})
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	for _, id := range []int64{a.ID, b.ID, c.ID} {
		_ = svc.Store.AddGroupMember(ctx, g.ID, id)
	}
	// Expense split only between A and B; C is a group member but not a participant.
	orig, _ := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL", ExpenseDate: "2026-01-10",
		Currency: "USD", PaidBy: a.ID, AddedBy: a.ID, GroupID: nullInt(&g.ID),
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})

	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD", ExpenseDate: "2026-01-10",
		PaidBy: a.ID, GroupID: &g.ID, Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}}, ActorID: c.ID,
	}
	if _, err := svc.MoveExpense(ctx, orig.ID, in, true); err != ErrNotEditor {
		t.Fatalf("group-member-non-participant edit: got %v, want ErrNotEditor", err)
	}
}

func TestMoveExpenseNotMovableWhenDeleted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	b, _ := svc.Store.CreateUser(ctx, &store.User{Name: "B", Email: "b@x.com"})
	orig, _ := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "X", Category: "general", Amount: 1000, SplitType: "EQUAL", ExpenseDate: "2026-01-10",
		Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	_ = svc.Store.SoftDeleteExpense(ctx, orig.ID, a.ID, orig.Version)

	in := ExpenseInput{
		Name: "X", Total: 1000, Method: split.EQUAL, Currency: "USD", ExpenseDate: "2026-01-10",
		PaidBy: a.ID, Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}}, ActorID: a.ID,
	}
	if _, err := svc.MoveExpense(ctx, orig.ID, in, true); err != ErrNotMovable {
		t.Fatalf("deleted expense: got %v, want ErrNotMovable", err)
	}
}
