package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// TestGenerateDueRecurrencesSkipsABrokenTemplate: a recurrence whose template
// can no longer recur -- a history collapse, scheduled before CreateRecurrence
// refused those -- generates nothing but is still advanced, so it is not
// retried on every tick, while a healthy recurrence due at the same time
// generates once. The next run finds nothing due.
func TestGenerateDueRecurrencesSkipsABrokenTemplate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	parts := []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}}
	for i, tmpl := range []*store.Expense{
		{Name: "Rent", SplitType: string(split.EQUAL)},
		{Name: "Historical Transactions", SplitType: string(split.ARCHIVE)},
	} {
		tmpl.Category, tmpl.Amount, tmpl.ExpenseDate, tmpl.Currency = "general", 1000, "2026-01-10", "USD"
		tmpl.PaidBy, tmpl.AddedBy = a.ID, a.ID
		created, err := svc.Store.CreateExpense(ctx, tmpl, parts)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
			CronExpression: "0 0 * * *", JobName: fmt.Sprintf("job-%d", i), TemplateExpenseID: created.ID,
			CreatedBy: a.ID, NextRunAt: sql.NullString{String: "2020-01-01T00:00:00.000Z", Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}

	if n, err := svc.GenerateDueRecurrences(ctx); err != nil || n != 1 {
		t.Fatalf("first run generated %d (err %v), want 1", n, err)
	}
	var names []string
	rows, err := svc.Store.DB.Query(`SELECT name FROM expenses WHERE recurrence_id IS NOT NULL`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	_ = rows.Close()
	if fmt.Sprint(names) != "[Rent]" {
		t.Errorf("generated %v, want only the healthy Rent", names)
	}
	if due, err := svc.Store.ListDueRecurrences(ctx, toISO(time.Now())); err != nil || len(due) != 0 {
		t.Errorf("still due after the run: %d (err %v), want both advanced", len(due), err)
	}
	if n, err := svc.GenerateDueRecurrences(ctx); err != nil || n != 0 {
		t.Errorf("second run generated %d (err %v), want 0", n, err)
	}
}
