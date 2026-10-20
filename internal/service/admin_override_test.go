package service

import (
	"context"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// overrideFixture is two members (a, b) of group g with a 10.00 EQUAL expense b
// paid inside it, plus an admin and a plain user (outsider) who are in neither
// the group nor the expense.
type overrideFixture struct {
	svc                   *Service
	a, b, admin, outsider *store.User
	g                     *store.Group
	exp                   *store.Expense
}

func newOverrideFixture(t *testing.T) overrideFixture {
	t.Helper()
	ctx := context.Background()
	svc, _ := newTestService(t)
	f := overrideFixture{svc: svc}
	f.a = covUser(t, svc, "A", "a@x.com")
	f.b = covUser(t, svc, "B", "b@x.com")
	f.outsider = covUser(t, svc, "Out", "out@x.com")
	admin, err := svc.Store.CreateUser(ctx, &store.User{Name: "Root", Email: "root@x.com", Role: store.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	f.admin = admin
	f.g, _ = svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: f.a.ID, DefaultCurrency: "USD"})
	_ = svc.Store.AddGroupMember(ctx, f.g.ID, f.b.ID)
	f.exp, err = svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Hotel", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-03-10", Currency: "USD", PaidBy: f.b.ID, AddedBy: f.b.ID, GroupID: nullInt(&f.g.ID),
	}, []store.ExpenseParticipant{{UserID: f.b.ID, Amount: 500}, {UserID: f.a.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// editInput is an unchanged re-save of the fixture's expense by actor into gid.
func (f overrideFixture) editInput(actor int64, gid *int64, name string) ExpenseInput {
	return ExpenseInput{
		Name: name, Category: "general", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-03-10", PaidBy: f.b.ID, GroupID: gid,
		Lines: []split.Line{{UserID: f.a.ID}, {UserID: f.b.ID}}, ActorID: actor, Version: f.exp.Version,
	}
}

// TestIsAdminFailsClosed pins who gets the override: an active admin, and no one
// else -- not a deactivated admin, not an id that does not exist, not USER or
// LIMITED. Each refused actor is an outsider to the expense, so CanEditExpense
// must say no for them too.
func TestIsAdminFailsClosed(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t)
	retired, _ := f.svc.Store.CreateUser(ctx, &store.User{Name: "Ex", Email: "ex@x.com", Role: store.RoleAdmin})
	if err := f.svc.Store.SetDeactivated(ctx, retired.ID, true); err != nil {
		t.Fatal(err)
	}
	limited := covLimitedUser(t, f.svc, "Lim", "lim@x.com")

	for _, tc := range []struct {
		name string
		id   int64
		want bool
	}{
		{"active admin", f.admin.ID, true},
		{"deactivated admin", retired.ID, false},
		{"unknown id", 99999, false},
		{"USER", f.outsider.ID, false},
		{"LIMITED", limited.ID, false},
	} {
		if got := f.svc.isAdmin(ctx, tc.id); got != tc.want {
			t.Errorf("isAdmin(%s) = %v, want %v", tc.name, got, tc.want)
		}
		if got := f.svc.CanEditExpense(ctx, tc.id, f.exp); got != tc.want {
			t.Errorf("CanEditExpense(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestAdminEditsAndDeletesAnyTransaction: an admin outside the group and the
// expense can edit it in place and delete it, each write is stamped with the
// admin, and a plain outsider still cannot do either.
func TestAdminEditsAndDeletesAnyTransaction(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t)
	if f.svc.IsTransactionMember(ctx, f.admin.ID, f.exp) {
		t.Fatal("fixture: the admin must not be part of the expense")
	}

	if _, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.outsider.ID, &f.g.ID, "Hijack"), false); err != ErrNotEditor {
		t.Fatalf("outsider edit: got %v, want ErrNotEditor", err)
	}
	edited, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.admin.ID, &f.g.ID, "Hotel (fixed)"), false)
	if err != nil {
		t.Fatalf("admin in-place edit: %v", err)
	}
	if edited.Name != "Hotel (fixed)" || !edited.UpdatedBy.Valid || edited.UpdatedBy.Int64 != f.admin.ID {
		t.Errorf("admin edit not applied or not stamped: name %q updated_by %+v", edited.Name, edited.UpdatedBy)
	}

	if err := f.svc.DeleteExpense(ctx, f.exp.ID, f.outsider.ID, edited.Version); err != ErrNotEditor {
		t.Fatalf("outsider delete: got %v, want ErrNotEditor", err)
	}
	if err := f.svc.DeleteExpense(ctx, f.exp.ID, f.admin.ID, edited.Version-1); err != store.ErrStaleExpense {
		t.Fatalf("admin stale delete: got %v, want ErrStaleExpense", err)
	}
	if err := f.svc.DeleteExpense(ctx, f.exp.ID, f.admin.ID, edited.Version); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	gone, _ := f.svc.Store.GetExpense(ctx, f.exp.ID)
	if !gone.DeletedAt.Valid || !gone.DeletedBy.Valid || gone.DeletedBy.Int64 != f.admin.ID {
		t.Errorf("delete not applied or not stamped with the admin: %+v / %+v", gone.DeletedAt, gone.DeletedBy)
	}
	// A deleted row stays read-only, for the admin too.
	if _, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.admin.ID, &f.g.ID, "Back"), false); err != ErrNotMovable {
		t.Errorf("admin edit of a deleted row: got %v, want ErrNotMovable", err)
	}
}

// TestAdminMoveSkipsOnlyTheActorCheck: moving into a group the admin is not in
// works when the payer and every participant belong to it, but not otherwise --
// and an admin who is the payer is checked as the payer.
func TestAdminMoveSkipsOnlyTheActorCheck(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t)
	both, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "Both", CreatedBy: f.a.ID, DefaultCurrency: "USD"})
	_ = f.svc.Store.AddGroupMember(ctx, both.ID, f.b.ID)
	onlyA, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "OnlyA", CreatedBy: f.a.ID, DefaultCurrency: "USD"})

	if _, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.admin.ID, &onlyA.ID, "Hotel"), true); err != ErrNotMember {
		t.Fatalf("admin move into a group missing the payer: got %v, want ErrNotMember", err)
	}
	moved, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.admin.ID, &both.ID, "Hotel"), true)
	if err != nil {
		t.Fatalf("admin move into a group with everyone involved: %v", err)
	}
	if moved.GroupID.Int64 != both.ID {
		t.Errorf("moved to group %d, want %d", moved.GroupID.Int64, both.ID)
	}

	// The admin paid this one and is not in Both: the payer check still applies.
	own, _ := f.svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Taxi", Category: "general", Amount: 600, SplitType: "EQUAL",
		ExpenseDate: "2026-03-11", Currency: "USD", PaidBy: f.admin.ID, AddedBy: f.admin.ID,
	}, []store.ExpenseParticipant{{UserID: f.admin.ID, Amount: 300}, {UserID: f.a.ID, Amount: -300}})
	in := ExpenseInput{
		Name: "Taxi", Category: "general", Total: 600, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-03-11", PaidBy: f.admin.ID, GroupID: &both.ID,
		Lines: []split.Line{{UserID: f.admin.ID}, {UserID: f.a.ID}}, ActorID: f.admin.ID, Version: own.Version,
	}
	if _, err := f.svc.MoveExpense(ctx, own.ID, in, true); err != ErrNotMember {
		t.Errorf("admin payer outside the target group: got %v, want ErrNotMember", err)
	}

	// The payer is in the target group but a non-payer participant is not.
	bOnly, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "BOnly", CreatedBy: f.b.ID, DefaultCurrency: "USD"})
	if _, err := f.svc.MoveExpense(ctx, f.exp.ID, f.editInput(f.admin.ID, &bOnly.ID, "Hotel"), true); err != ErrNotMember {
		t.Errorf("admin move with a participant outside the target group: got %v, want ErrNotMember", err)
	}

	// The admin is a (non-paying) participant and not in the target group.
	shared, _ := f.svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Lunch", Category: "general", Amount: 800, SplitType: "EQUAL",
		ExpenseDate: "2026-03-12", Currency: "USD", PaidBy: f.a.ID, AddedBy: f.a.ID,
	}, []store.ExpenseParticipant{{UserID: f.a.ID, Amount: 400}, {UserID: f.admin.ID, Amount: -400}})
	in = ExpenseInput{
		Name: "Lunch", Category: "general", Total: 800, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-03-12", PaidBy: f.a.ID, GroupID: &both.ID,
		Lines: []split.Line{{UserID: f.a.ID}, {UserID: f.admin.ID}}, ActorID: f.admin.ID, Version: shared.Version,
	}
	if _, err := f.svc.MoveExpense(ctx, shared.ID, in, true); err != ErrNotMember {
		t.Errorf("admin participant outside the target group: got %v, want ErrNotMember", err)
	}
}

// TestAdminSettlementEditSkipsOnlyTheActorCheck is the same rule for a
// settlement: the pair must be members of the target group, the admin need not.
func TestAdminSettlementEditSkipsOnlyTheActorCheck(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t)
	onlyA, _ := f.svc.Store.CreateGroup(ctx, &store.Group{Name: "OnlyA", CreatedBy: f.a.ID, DefaultCurrency: "USD"})
	st, err := f.svc.Settle(ctx, f.a.ID, f.b.ID, 500, "USD", nil, "2026-03-12", f.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := SettlementInput{Name: "Settlement", Amount: 500, Currency: "USD", ExpenseDate: "2026-03-12",
		GroupID: &onlyA.ID, ActorID: f.admin.ID, Version: st.Version}
	if _, err := f.svc.UpdateSettlement(ctx, st.ID, in, true); err != ErrNotMember {
		t.Fatalf("admin settlement move with the counterparty outside: got %v, want ErrNotMember", err)
	}
	in.GroupID = &f.g.ID
	moved, err := f.svc.UpdateSettlement(ctx, st.ID, in, true)
	if err != nil {
		t.Fatalf("admin settlement move into the pair's group: %v", err)
	}
	if moved.GroupID.Int64 != f.g.ID {
		t.Errorf("settlement in group %d, want %d", moved.GroupID.Int64, f.g.ID)
	}
	in.ActorID, in.GroupID, in.Version = f.outsider.ID, nil, moved.Version
	if _, err := f.svc.UpdateSettlement(ctx, st.ID, in, true); err != ErrNotEditor {
		t.Errorf("outsider settlement edit: got %v, want ErrNotEditor", err)
	}

	// The admin is one side of this settlement and not in g: the pair check
	// still applies to them. (A direct settlement needs the two to be related.)
	covFriends(t, f.svc, f.admin, f.a)
	own, err := f.svc.Settle(ctx, f.admin.ID, f.a.ID, 300, "USD", nil, "2026-03-13", f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	in = SettlementInput{Name: "Settlement", Amount: 300, Currency: "USD", ExpenseDate: "2026-03-13",
		GroupID: &f.g.ID, ActorID: f.admin.ID, Version: own.Version}
	if _, err := f.svc.UpdateSettlement(ctx, own.ID, in, true); err != ErrNotMember {
		t.Errorf("admin settlement party outside the target group: got %v, want ErrNotMember", err)
	}
}

// TestAdminGetsNoGroupLevelPowers: the override covers existing transactions
// only. An admin outside a group still cannot add an expense to it or collapse
// its history, and nothing is written.
func TestAdminGetsNoGroupLevelPowers(t *testing.T) {
	ctx := context.Background()
	f := newOverrideFixture(t)
	before := countRows(t, f.svc, "expenses")
	in := ExpenseInput{
		Name: "Sneak", Category: "general", Total: 400, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-03-13", PaidBy: f.a.ID, GroupID: &f.g.ID,
		Lines: []split.Line{{UserID: f.a.ID}, {UserID: f.b.ID}}, ActorID: f.admin.ID,
	}
	if _, err := f.svc.AddExpense(ctx, in); err != ErrNotMember {
		t.Errorf("admin add to a group they are not in: got %v, want ErrNotMember", err)
	}
	if n := countRows(t, f.svc, "expenses"); n != before {
		t.Errorf("expenses %d -> %d after a refused add", before, n)
	}
	if _, err := f.svc.ArchiveGroup(ctx, f.admin.ID, f.g.ID, "2099-01-01"); err != ErrNotGroupMember {
		t.Errorf("admin collapse of a group they are not in: got %v, want ErrNotGroupMember", err)
	}
}
