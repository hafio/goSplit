package store

import (
	"context"
	"errors"
	"testing"
)

// seedSplitInputExpense creates a two-person expense and returns it.
func seedSplitInputExpense(t *testing.T, st *Store) *Expense {
	t.Helper()
	ctx := context.Background()
	a := covUser(t, st, "alice")
	b := covUser(t, st, "bob")
	e, err := st.CreateExpense(ctx, &Expense{
		Name: "Dinner", Category: "general", Amount: 3000, SplitType: "SHARE",
		ExpenseDate: "2026-01-02", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []ExpenseParticipant{{UserID: a.ID, Amount: 1000}, {UserID: b.ID, Amount: -1000}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return e
}

// TestSplitInputsRoundTrip: a payload stored for an expense reads back verbatim,
// a second write replaces it, and an expense with none reports "".
func TestSplitInputsRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)

	if got, err := st.GetSplitInputs(ctx, e.ID); err != nil || got != "" {
		t.Fatalf("unset inputs = %q, %v; want empty", got, err)
	}
	const first = `{"v":1,"method":"SHARE","values":{"1":2,"2":1}}`
	if err := st.PutSplitInputs(ctx, e.ID, first); err != nil {
		t.Fatalf("put: %v", err)
	}
	if got, _ := st.GetSplitInputs(ctx, e.ID); got != first {
		t.Errorf("inputs = %q, want %q", got, first)
	}
	const second = `{"v":1,"method":"SHARE","values":{"1":3,"2":1}}`
	if err := st.PutSplitInputs(ctx, e.ID, second); err != nil {
		t.Fatalf("re-put: %v", err)
	}
	if got, _ := st.GetSplitInputs(ctx, e.ID); got != second {
		t.Errorf("inputs after overwrite = %q, want %q", got, second)
	}
}

// TestUpdateExpenseClearsSplitInputs: the stored inputs describe the split being
// replaced, so an edit drops them rather than leaving a stale restore behind.
func TestUpdateExpenseClearsSplitInputs(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)
	if err := st.PutSplitInputs(ctx, e.ID, `{"v":1,"method":"SHARE","values":{"1":2,"2":1}}`); err != nil {
		t.Fatalf("put: %v", err)
	}
	e.Amount = 4000
	if err := st.UpdateExpense(ctx, e, []ExpenseParticipant{
		{UserID: 1, Amount: 2000}, {UserID: 2, Amount: -2000},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := st.GetSplitInputs(ctx, e.ID); got != "" {
		t.Errorf("inputs survived an edit: %q", got)
	}
}

// TestSplitInputsCascadeOnDelete: the sidecar is tied to its expense, so a hard
// delete (the collapse path) takes it with it.
func TestSplitInputsCascadeOnDelete(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)
	if err := st.PutSplitInputs(ctx, e.ID, `{"v":1,"method":"SHARE","values":{"1":1}}`); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := st.DB.ExecContext(ctx, st.rebind(`DELETE FROM expenses WHERE id = ?`), e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := st.DB.QueryRowContext(ctx, st.rebind(
		`SELECT count(*) FROM expense_split_inputs WHERE expense_id = ?`), e.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d orphaned split-input rows after the expense was deleted", n)
	}
}

// TestUpdateExpenseVersionGuard: a save from a stale form is refused, and -- the
// part that matters -- it leaves the winner's participant rows intact rather
// than deleting them on the way to failing.
func TestUpdateExpenseVersionGuard(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)
	if e.Version != InitialVersion {
		t.Fatalf("new expense version = %d, want %d", e.Version, InitialVersion)
	}

	// Two editors read the same row.
	first, _ := st.GetExpense(ctx, e.ID)
	stale, _ := st.GetExpense(ctx, e.ID)

	first.Name = "Winner"
	if err := st.UpdateExpense(ctx, first, []ExpenseParticipant{
		{UserID: 1, Amount: 1500}, {UserID: 2, Amount: -1500},
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}
	bumped, _ := st.GetExpense(ctx, e.ID)
	if bumped.Version != InitialVersion+1 {
		t.Errorf("version after update = %d, want %d", bumped.Version, InitialVersion+1)
	}

	stale.Name = "Loser"
	err := st.UpdateExpense(ctx, stale, []ExpenseParticipant{
		{UserID: 1, Amount: 900}, {UserID: 2, Amount: -900},
	})
	if !errors.Is(err, ErrStaleExpense) {
		t.Fatalf("stale update err = %v, want ErrStaleExpense", err)
	}
	after, _ := st.GetExpense(ctx, e.ID)
	if after.Name != "Winner" || after.Version != InitialVersion+1 {
		t.Errorf("stale update changed the row: %+v", after)
	}
	parts, _ := st.GetParticipants(ctx, e.ID)
	if len(parts) != 2 || parts[0].Amount != 1500 {
		t.Errorf("stale update disturbed the participant rows: %+v", parts)
	}
}

// TestSoftDeleteBumpsVersion: deleting an expense invalidates any edit form
// already open on it, so a concurrent save cannot resurrect it.
func TestSoftDeleteBumpsVersion(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)
	open, _ := st.GetExpense(ctx, e.ID)

	if err := st.SoftDeleteExpense(ctx, e.ID, 1, e.Version); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	open.Name = "Edited after delete"
	err := st.UpdateExpense(ctx, open, []ExpenseParticipant{
		{UserID: 1, Amount: 1000}, {UserID: 2, Amount: -1000},
	})
	if !errors.Is(err, ErrStaleExpense) {
		t.Fatalf("edit after delete err = %v, want ErrStaleExpense", err)
	}
}

// TestSoftDeleteRejectsStaleVersion is the mirror of TestSoftDeleteBumpsVersion:
// an edit that lands after the delete page was rendered makes that delete stale,
// so it is refused rather than discarding a split the deleter never saw. A
// second delete of the same row is refused for the same reason.
func TestSoftDeleteRejectsStaleVersion(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	e := seedSplitInputExpense(t, st)
	open, _ := st.GetExpense(ctx, e.ID) // the version the delete button carries

	// Someone else saves an edit first.
	edit, _ := st.GetExpense(ctx, e.ID)
	edit.Name = "Winner"
	if err := st.UpdateExpense(ctx, edit, []ExpenseParticipant{
		{UserID: 1, Amount: 1500}, {UserID: 2, Amount: -1500},
	}); err != nil {
		t.Fatalf("competing edit: %v", err)
	}

	if err := st.SoftDeleteExpense(ctx, e.ID, 1, open.Version); !errors.Is(err, ErrStaleExpense) {
		t.Fatalf("stale delete err = %v, want ErrStaleExpense", err)
	}
	live, _ := st.GetExpense(ctx, e.ID)
	if live.DeletedAt.Valid {
		t.Error("a stale delete removed the row anyway")
	}
	if live.Name != "Winner" {
		t.Errorf("stale delete disturbed the winner's edit: %q", live.Name)
	}

	// With the current version it goes through -- and only once.
	if err := st.SoftDeleteExpense(ctx, e.ID, 1, live.Version); err != nil {
		t.Fatalf("current-version delete: %v", err)
	}
	if err := st.SoftDeleteExpense(ctx, e.ID, 1, live.Version); !errors.Is(err, ErrStaleExpense) {
		t.Errorf("repeat delete err = %v, want ErrStaleExpense", err)
	}
}
