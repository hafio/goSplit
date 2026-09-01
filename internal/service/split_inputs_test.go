package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// TestAddExpenseRecordsTypedInputs: what reaches storage is what the user typed
// (share weights), not the minor-unit amounts those weights produced -- the
// distinction the old reconstruct-from-amounts path could not make.
func TestAddExpenseRecordsTypedInputs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	e, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Dinner", Total: 3000, Method: split.SHARE, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{
			split.LineFromInput(split.SHARE, a.ID, 2),
			split.LineFromInput(split.SHARE, b.ID, 1),
		},
	})
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	payload, err := svc.Store.GetSplitInputs(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetSplitInputs: %v", err)
	}
	values, ok := split.DecodeInputs(payload, split.SHARE)
	if !ok {
		t.Fatalf("payload %q did not decode", payload)
	}
	if values[a.ID] != 2 || values[b.ID] != 1 {
		t.Errorf("stored inputs = %v, want the typed weights {A:2, B:1}", values)
	}
}

// TestAddExpenseOmitsNonSplittingPayer: the creditor row the split engine forces
// in for a payer who owes nothing records no input, which is how the edit form
// knows to leave them unchecked.
func TestAddExpenseOmitsNonSplittingPayer(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	e, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Treat", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: b.ID}},
	})
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	payload, _ := svc.Store.GetSplitInputs(ctx, e.ID)
	values, ok := split.DecodeInputs(payload, split.EQUAL)
	if !ok {
		t.Fatalf("payload %q did not decode", payload)
	}
	if _, present := values[a.ID]; present {
		t.Errorf("payer who owes nothing recorded an input: %v", values)
	}
	if _, present := values[b.ID]; !present {
		t.Errorf("the actual splitter is missing from %v", values)
	}
}

// TestSettleRecordsNoInputs: settlements never run through the split engine, so
// there is nothing to record and the edit form keeps its old behaviour.
func TestSettleRecordsNoInputs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	e, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if payload, _ := svc.Store.GetSplitInputs(ctx, e.ID); payload != "" {
		t.Errorf("settlement recorded split inputs: %q", payload)
	}
}

// TestUpdateSettlementKeepsSplitType is the regression guard for the bug this
// path exists to fix: editing a settlement must not turn it into an ordinary
// expense.
func TestUpdateSettlementKeepsSplitType(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	s0, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	updated, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 4000, Currency: "USD", ExpenseDate: "2026-01-03",
		Note: "corrected", ActorID: a.ID, Version: s0.Version,
	}, false)
	if err != nil {
		t.Fatalf("UpdateSettlement: %v", err)
	}
	if updated.SplitType != string(split.SETTLEMENT) {
		t.Fatalf("split type = %q, want SETTLEMENT", updated.SplitType)
	}
	if updated.Amount != 4000 || updated.Note != "corrected" {
		t.Errorf("edit not applied: %+v", updated)
	}
	parts, _ := svc.Store.GetParticipants(ctx, s0.ID)
	if len(parts) != 2 {
		t.Fatalf("participant rows = %d, want 2", len(parts))
	}
	byUser := map[int64]int64{}
	for _, p := range parts {
		byUser[p.UserID] = p.Amount
	}
	if byUser[a.ID] != 4000 || byUser[b.ID] != -4000 {
		t.Errorf("rows = %v, want payer +4000 / counterparty -4000", byUser)
	}
}

// TestUpdateSettlementGuards covers the refusals: a non-settlement, a bad
// amount, an unrelated actor, and a stale version.
func TestUpdateSettlementGuards(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	c := covUser(t, svc, "C", "c@x.com")

	ordinary, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	})
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	if _, err := svc.UpdateSettlement(ctx, ordinary.ID, SettlementInput{
		Amount: 100, ActorID: a.ID,
	}, false); !errors.Is(err, ErrNotSettlement) {
		t.Errorf("ordinary expense: err = %v, want ErrNotSettlement", err)
	}

	s0, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 0, ActorID: a.ID, Version: s0.Version,
	}, false); !errors.Is(err, ErrSettlementAmount) {
		t.Errorf("zero amount: err = %v, want ErrSettlementAmount", err)
	}
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 100, ActorID: c.ID, Version: s0.Version,
	}, false); !errors.Is(err, ErrNotEditor) {
		t.Errorf("outsider: err = %v, want ErrNotEditor", err)
	}
	// A save from a form rendered before someone else's edit is refused.
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 3000, Currency: "USD", ActorID: a.ID, Version: s0.Version,
	}, false); err != nil {
		t.Fatalf("first edit: %v", err)
	}
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 9999, Currency: "USD", ActorID: a.ID, Version: s0.Version,
	}, false); !errors.Is(err, store.ErrStaleExpense) {
		t.Errorf("stale edit: err = %v, want ErrStaleExpense", err)
	}
}

// TestMoveExpenseRejectsStaleVersion: the same guard on the ordinary edit path.
func TestMoveExpenseRejectsStaleVersion(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	}
	e, err := svc.AddExpense(ctx, in)
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	edit := in
	edit.Name = "Winner"
	edit.Version = e.Version
	if _, err := svc.MoveExpense(ctx, e.ID, edit, false); err != nil {
		t.Fatalf("first edit: %v", err)
	}
	stale := in
	stale.Name = "Loser"
	stale.Version = e.Version // the version the second form was rendered with
	if _, err := svc.MoveExpense(ctx, e.ID, stale, false); !errors.Is(err, store.ErrStaleExpense) {
		t.Fatalf("stale edit: err = %v, want ErrStaleExpense", err)
	}
	got, _ := svc.Store.GetExpense(ctx, e.ID)
	if got.Name != "Winner" {
		t.Errorf("stale edit overwrote the winner: %q", got.Name)
	}
}

// TestGenerateOneReSplitsFromInputs: a recurrence whose template records inputs
// re-splits for the new date and carries the inputs forward, so the generated
// copy is editable with the same fidelity. Its amounts must stay consistent with
// those inputs -- the reason it recomputes instead of copying the rows.
func TestGenerateOneReSplitsFromInputs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	tmpl, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Rent", Total: 3000, Method: split.SHARE, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{
			split.LineFromInput(split.SHARE, a.ID, 2),
			split.LineFromInput(split.SHARE, b.ID, 1),
		},
	})
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	rec, err := svc.CreateRecurrence(ctx, a.ID, tmpl.ID, "0 0 * * *")
	if err != nil {
		t.Fatalf("CreateRecurrence: %v", err)
	}
	if err := svc.generateOne(ctx, rec); err != nil {
		t.Fatalf("generateOne: %v", err)
	}
	gen := generatedFrom(t, svc, tmpl.ID)
	payload, _ := svc.Store.GetSplitInputs(ctx, gen.ID)
	values, ok := split.DecodeInputs(payload, split.SHARE)
	if !ok {
		t.Fatalf("generated expense carries no inputs (%q)", payload)
	}
	if values[a.ID] != 2 || values[b.ID] != 1 {
		t.Errorf("carried inputs = %v, want {A:2, B:1}", values)
	}
	// The stored amounts must be what those weights produce on the new date.
	want, err := split.Compute(split.SHARE, gen.Amount, gen.PaidBy, gen.ExpenseDate, []split.Line{
		split.LineFromInput(split.SHARE, a.ID, 2),
		split.LineFromInput(split.SHARE, b.ID, 1),
	})
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	got, _ := svc.Store.GetParticipants(ctx, gen.ID)
	byUser := map[int64]int64{}
	for _, p := range got {
		byUser[p.UserID] = p.Amount
	}
	for _, w := range want {
		if byUser[w.UserID] != w.Amount {
			t.Errorf("user %d amount = %d, want %d (inputs and amounts disagree)",
				w.UserID, byUser[w.UserID], w.Amount)
		}
	}
}

// TestGenerateOneCopiesWhenNoInputs: a template with nothing recorded keeps the
// original verbatim-copy behaviour.
func TestGenerateOneCopiesWhenNoInputs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	tmpl, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Rent", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	rec, err := svc.CreateRecurrence(ctx, a.ID, tmpl.ID, "0 0 * * *")
	if err != nil {
		t.Fatalf("CreateRecurrence: %v", err)
	}
	if err := svc.generateOne(ctx, rec); err != nil {
		t.Fatalf("generateOne: %v", err)
	}
	gen := generatedFrom(t, svc, tmpl.ID)
	parts, _ := svc.Store.GetParticipants(ctx, gen.ID)
	byUser := map[int64]int64{}
	for _, p := range parts {
		byUser[p.UserID] = p.Amount
	}
	if byUser[a.ID] != 500 || byUser[b.ID] != -500 {
		t.Errorf("rows = %v, want the template's +500/-500 copied verbatim", byUser)
	}
	if payload, _ := svc.Store.GetSplitInputs(ctx, gen.ID); payload != "" {
		t.Errorf("generated expense invented inputs: %q", payload)
	}
}

// TestUpdateSettlementRequiresTargetGroupMembership: the target group is checked
// for the actor as well as for the pair. Settling a whole group makes the acting
// member added_by on transfers between two other people, and CanEditExpense then
// grants them the edit -- without the actor check they could file that
// settlement into a group they do not belong to, where balances bucket by
// group_id and would move for members who never took part.
func TestUpdateSettlementRequiresTargetGroupMembership(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com") // recorded it; not a member of either group
	b := covUser(t, svc, "B", "b@x.com")
	c := covUser(t, svc, "C", "c@x.com")
	// CreateGroup adds the creator, so B is a member of both; C only of shared.
	shared, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Shared", CreatedBy: b.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := svc.Store.AddGroupMember(ctx, shared.ID, c.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}
	solo, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Solo", CreatedBy: b.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// A settlement between B and C that A recorded, currently in no group.
	s0, err := svc.Settle(ctx, b.ID, c.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	move := func(actor, groupID int64) error {
		_, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
			Amount: 2500, Currency: "USD", GroupID: &groupID, ActorID: actor, Version: s0.Version,
		}, true)
		return err
	}
	// A is the creator, so CanEditExpense lets them in -- but not into a group
	// they are not part of.
	if err := move(a.ID, shared.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("actor outside the target group: err = %v, want ErrNotMember", err)
	}
	// The pair is still checked too: B belongs to Solo, C does not.
	if err := move(b.ID, solo.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("counterparty outside the target group: err = %v, want ErrNotMember", err)
	}
	// Everyone involved is in Shared, so the same move goes through there.
	if err := move(b.ID, shared.ID); err != nil {
		t.Fatalf("member actor moving into a shared group: %v", err)
	}
	got, _ := svc.Store.GetExpense(ctx, s0.ID)
	if !got.GroupID.Valid || got.GroupID.Int64 != shared.ID {
		t.Errorf("settlement group = %+v, want %d", got.GroupID, shared.ID)
	}
}

// TestMoveExpenseRequiresActorInTargetGroup: the same gap on the ordinary edit
// path. Creator rights on a transaction are not membership of the group it is
// being filed into.
func TestMoveExpenseRequiresActorInTargetGroup(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com") // creator, not a member of G
	b := covUser(t, svc, "B", "b@x.com")
	c := covUser(t, svc, "C", "c@x.com")
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: b.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := svc.Store.AddGroupMember(ctx, g.ID, c.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}
	orig, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Dinner", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: b.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: b.ID, Amount: 500}, {UserID: c.ID, Amount: -500}})
	if err != nil {
		t.Fatalf("create expense: %v", err)
	}
	in := ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: b.ID, GroupID: &g.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: b.ID}, {UserID: c.ID}}, Version: orig.Version,
	}
	if _, err := svc.MoveExpense(ctx, orig.ID, in, true); !errors.Is(err, ErrNotMember) {
		t.Fatalf("actor outside the target group: err = %v, want ErrNotMember", err)
	}
	if err := svc.Store.AddGroupMember(ctx, g.ID, a.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if _, err := svc.MoveExpense(ctx, orig.ID, in, true); err != nil {
		t.Fatalf("member actor: %v", err)
	}
}

// TestUpdateSettlementRejectsCurrencySwitch: balances bucket by currency, so
// re-denominating a settlement would leave the debt it cleared outstanding and
// invent an offsetting one in the new currency. The currency is fixed for the
// life of the record; a form that carries none keeps the stored one rather than
// being read as a switch.
func TestUpdateSettlementRejectsCurrencySwitch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	s0, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 2500, Currency: "EUR", ActorID: a.ID, Version: s0.Version,
	}, false); !errors.Is(err, ErrSettlementCurrency) {
		t.Fatalf("currency switch: err = %v, want ErrSettlementCurrency", err)
	}
	kept, _ := svc.Store.GetExpense(ctx, s0.ID)
	if kept.Currency != "USD" || kept.Version != s0.Version {
		t.Errorf("a refused edit still touched the row: %+v", kept)
	}

	updated, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 3000, ActorID: a.ID, Version: s0.Version,
	}, false)
	if err != nil {
		t.Fatalf("edit carrying no currency: %v", err)
	}
	if updated.Currency != "USD" || updated.Amount != 3000 {
		t.Errorf("edit carrying no currency = %s/%d, want USD/3000", updated.Currency, updated.Amount)
	}
}

// TestMoveExpenseRejectsSettlement: the ordinary edit path re-splits through the
// split engine, which would rewrite split_type and quietly turn a settlement
// into a normal expense. The handler routes settlements to UpdateSettlement;
// this is the guard that holds if it ever stops.
func TestMoveExpenseRejectsSettlement(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	s0, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	in := ExpenseInput{
		Name: "Settlement", Total: 2500, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID, Version: s0.Version,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	}
	if _, err := svc.MoveExpense(ctx, s0.ID, in, false); !errors.Is(err, ErrIsSettlement) {
		t.Fatalf("settlement through the expense path: err = %v, want ErrIsSettlement", err)
	}
	got, _ := svc.Store.GetExpense(ctx, s0.ID)
	if got.SplitType != string(split.SETTLEMENT) {
		t.Errorf("split_type = %q, want SETTLEMENT", got.SplitType)
	}
}

// TestUpdateSettlementRejectsCorruptRows covers both ErrBadSettlement branches.
// A settlement is exactly two rows, one of them the payer's, by construction --
// anything else makes the direction unrecoverable, and editing it would have to
// guess which way the money went.
func TestUpdateSettlementRejectsCorruptRows(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	c := covUser(t, svc, "C", "c@x.com")

	edit := func(id string, version int64) error {
		_, err := svc.UpdateSettlement(ctx, id, SettlementInput{
			Amount: 100, Currency: "USD", ActorID: a.ID, Version: version,
		}, false)
		return err
	}

	// Wrong row count.
	s1, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-02", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if _, err := svc.Store.DB.Exec(
		`INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?, ?, 0)`,
		s1.ID, c.ID); err != nil {
		t.Fatalf("add a third row: %v", err)
	}
	if err := edit(s1.ID, s1.Version); !errors.Is(err, ErrBadSettlement) {
		t.Errorf("three participant rows: err = %v, want ErrBadSettlement", err)
	}

	// Two rows, but neither is the payer's.
	s2, err := svc.Settle(ctx, a.ID, b.ID, 2500, "USD", nil, "2026-01-03", a.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if _, err := svc.Store.DB.Exec(
		`UPDATE expense_participants SET user_id = ? WHERE expense_id = ? AND user_id = ?`,
		c.ID, s2.ID, a.ID); err != nil {
		t.Fatalf("rewrite the payer row: %v", err)
	}
	if err := edit(s2.ID, s2.Version); !errors.Is(err, ErrBadSettlement) {
		t.Errorf("payer missing from the rows: err = %v, want ErrBadSettlement", err)
	}
}

// TestRecordSplitInputsLogsFailure: the payload is reference data, so a failed
// write must not fail the save the user already completed -- but it must not
// vanish either, because the only other symptom is an edit form that quietly
// falls back to values derived from the amounts.
func TestRecordSplitInputsLogsFailure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	in := ExpenseInput{
		Name: "Dinner", Total: 3000, Method: split.SHARE, Currency: "USD",
		ExpenseDate: "2026-01-02", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{
			split.LineFromInput(split.SHARE, a.ID, 2),
			split.LineFromInput(split.SHARE, b.ID, 1),
		},
	}
	e, err := svc.AddExpense(ctx, in)
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}

	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// Take the sidecar table away so the write cannot succeed.
	if _, err := svc.Store.DB.Exec(`DROP TABLE expense_split_inputs`); err != nil {
		t.Fatalf("drop sidecar table: %v", err)
	}
	svc.recordSplitInputs(ctx, e.ID, in) // best-effort: reports nothing to the caller
	out := logged.String()
	if !strings.Contains(out, "split inputs not recorded") {
		t.Errorf("a failed split-inputs write was swallowed; log = %q", out)
	}
	if !strings.Contains(out, e.ID) {
		t.Errorf("the warning does not say which expense failed: %q", out)
	}
}

// generatedFrom returns the one expense a recurrence produced (i.e. not the
// template itself).
func generatedFrom(t *testing.T, svc *Service, tmplID string) *store.Expense {
	t.Helper()
	var id string
	err := svc.Store.DB.QueryRow(
		`SELECT id FROM expenses WHERE recurrence_id IS NOT NULL AND id <> ?`, tmplID).Scan(&id)
	if err != nil {
		t.Fatalf("find generated expense: %v", err)
	}
	e, err := svc.Store.GetExpense(context.Background(), id)
	if err != nil {
		t.Fatalf("load generated expense: %v", err)
	}
	return e
}
