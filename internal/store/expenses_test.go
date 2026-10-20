package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

// TestUpdateExpenseIsAllOrNothing: when the participant rewrite fails part way
// -- here a participant who does not exist -- the edit rolls back whole. The
// row keeps its fields and version, the old participants stay, and so do the
// split inputs the first statement had already deleted.
func TestUpdateExpenseIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	e := covDirectExpense(t, st, "Dinner", "2025-01-01", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err := st.PutSplitInputs(ctx, e.ID, `{"1":1,"2":1}`); err != nil {
		t.Fatal(err)
	}
	beforeParts, _ := st.GetParticipants(ctx, e.ID)

	edited := *e
	edited.Name = "Renamed"
	err := st.UpdateExpense(ctx, &edited, []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: 99999, Amount: -500}})
	if err == nil || errors.Is(err, ErrStaleExpense) {
		t.Fatalf("an update naming a missing participant = %v, want the insert to fail", err)
	}
	got, err := st.GetExpense(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Dinner" || got.Version != e.Version {
		t.Errorf("row after the failed update = %q v%d, want %q v%d", got.Name, got.Version, "Dinner", e.Version)
	}
	if parts, _ := st.GetParticipants(ctx, e.ID); fmt.Sprint(parts) != fmt.Sprint(beforeParts) {
		t.Errorf("participants after the failed update = %v, want %v", parts, beforeParts)
	}
	if payload, _ := st.GetSplitInputs(ctx, e.ID); payload != `{"1":1,"2":1}` {
		t.Errorf("split inputs after the failed update = %q, want them kept", payload)
	}
}

// TestCollapseToHistoricalIsAllOrNothing: when an origin cannot be deleted --
// here a recurrence still uses it as its template -- the collapse rolls back
// whole: no synthetic row, no archive copies, and every original stays live.
func TestCollapseToHistoricalIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "A")
	b := covUser(t, st, "B")
	e1 := covDirectExpense(t, st, "old1", "2025-01-01", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 300}, {UserID: b.ID, Amount: -300}})
	e2 := covDirectExpense(t, st, "old2", "2025-01-02", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 200}, {UserID: b.ID, Amount: -200}})
	if _, err := st.CreateRecurrence(ctx, &ExpenseRecurrence{
		CronExpression: "0 0 * * *", JobName: "job-1", TemplateExpenseID: e2.ID, CreatedBy: a.ID,
		NextRunAt: sql.NullString{String: "2025-02-01T00:00:00.000Z", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	synthetic := &Expense{Name: "Historical Transactions", Category: "general", Amount: 500, SplitType: "EQUAL",
		ExpenseDate: "2025-01-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
	err := st.CollapseToHistorical(ctx, []HistoricalBatch{{
		Expense:      synthetic,
		Participants: []ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}},
		OriginIDs:    []string{e1.ID, e2.ID},
	}})
	if err == nil {
		t.Fatal("a collapse whose origin cannot be deleted succeeded")
	}
	for _, id := range []string{e1.ID, e2.ID} {
		if _, err := st.GetExpense(ctx, id); err != nil {
			t.Errorf("origin %s is no longer live: %v", id, err)
		}
	}
	if synthetic.ID != "" {
		if _, err := st.GetExpense(ctx, synthetic.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("the synthetic row survived the rollback (err %v)", err)
		}
	}
	for _, table := range []string{"archived_expenses", "archived_expense_participants"} {
		var n int
		if err := st.DB.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s holds %d rows after the rollback (err %v)", table, n, err)
		}
	}
}

// TestSoftDeleteConversionTakesBothLegs: deleting either leg of a conversion
// deletes the pair together, version-checked on the leg chosen -- a stale
// version deletes neither -- and leaves any other conversion alone.
func TestSoftDeleteConversionTakesBothLegs(t *testing.T) {
	ctx := context.Background()
	for _, pick := range []string{"from", "to"} {
		t.Run(pick, func(t *testing.T) {
			st := openTestStore(t)
			a := covUser(t, st, "A")
			b := covUser(t, st, "B")
			pair := func() (string, string) {
				t.Helper()
				from := &Expense{Name: "conv from", Category: "general", Amount: 10000, SplitType: "CURRENCY_CONVERSION",
					ExpenseDate: "2025-02-01", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID}
				to := &Expense{Name: "conv to", Category: "general", Amount: 9000, SplitType: "CURRENCY_CONVERSION",
					ExpenseDate: "2025-02-01", Currency: "EUR", PaidBy: b.ID, AddedBy: b.ID}
				fromID, toID, err := st.CreateConversionPair(ctx,
					from, []ExpenseParticipant{{UserID: a.ID, Amount: 10000}, {UserID: b.ID, Amount: -10000}},
					to, []ExpenseParticipant{{UserID: a.ID, Amount: -9000}, {UserID: b.ID, Amount: 9000}})
				if err != nil {
					t.Fatal(err)
				}
				return fromID, toID
			}
			fromID, toID := pair()
			otherFrom, otherTo := pair()
			id := fromID
			if pick == "to" {
				id = toID
			}
			deleted := func(id string) bool {
				t.Helper()
				e, err := st.GetExpense(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				return e.DeletedAt.Valid
			}
			chosen, err := st.GetExpense(ctx, id)
			if err != nil {
				t.Fatal(err)
			}

			if err := st.SoftDeleteConversion(ctx, id, a.ID, chosen.Version+1); !errors.Is(err, ErrStaleExpense) {
				t.Fatalf("delete with a stale version = %v, want ErrStaleExpense", err)
			}
			if deleted(fromID) || deleted(toID) {
				t.Fatal("a stale delete removed a leg")
			}
			if err := st.SoftDeleteConversion(ctx, id, a.ID, chosen.Version); err != nil {
				t.Fatal(err)
			}
			if !deleted(fromID) || !deleted(toID) {
				t.Errorf("deleting the %s leg left the pair half applied: from deleted %v, to deleted %v", pick, deleted(fromID), deleted(toID))
			}
			if deleted(otherFrom) || deleted(otherTo) {
				t.Error("another conversion was deleted too")
			}
		})
	}
}
