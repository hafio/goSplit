package service

import (
	"context"
	"testing"
)

func TestCreateConversionExact(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Register(ctx, "A", "a@example.com", "password12")
	b, _ := svc.Register(ctx, "B", "b@example.com", "password12")
	covFriends(t, svc, a, b)

	// Explicit amounts are stored verbatim — no rate recomputation.
	e, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 10000, 7830, "SGD", "USD", "2026-01-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Amount != 10000 || e.Currency != "SGD" || !e.ConversionToID.Valid {
		t.Fatalf("from-leg = %d/%s link=%v", e.Amount, e.Currency, e.ConversionToID.Valid)
	}
	to, err := svc.Store.GetExpense(ctx, e.ConversionToID.String)
	if err != nil {
		t.Fatal(err)
	}
	if to.Amount != 7830 || to.Currency != "USD" {
		t.Fatalf("to-leg = %d/%s, want 7830/USD", to.Amount, to.Currency)
	}

	// Validation.
	if _, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 100, 100, "USD", "USD", "", nil); err == nil {
		t.Error("same currency should error")
	}
	if _, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 0, 100, "SGD", "USD", "", nil); err == nil {
		t.Error("zero from-amount should error")
	}
	if _, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 100, -5, "SGD", "USD", "", nil); err == nil {
		t.Error("negative to-amount should error")
	}
}

// TestCreateConversionExactDatesABlankDateToday: a conversion entered without a
// date is recorded on today's date, on both legs.
func TestCreateConversionExactDatesABlankDateToday(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a, _ := svc.Register(ctx, "A", "a@example.com", "password12")
	b, _ := svc.Register(ctx, "B", "b@example.com", "password12")
	covFriends(t, svc, a, b)

	before := todayISO()
	e, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 10000, 7830, "SGD", "USD", "", nil)
	after := todayISO()
	if err != nil {
		t.Fatal(err)
	}
	to, err := svc.Store.GetExpense(ctx, e.ConversionToID.String)
	if err != nil {
		t.Fatal(err)
	}
	for _, leg := range []string{e.ExpenseDate, to.ExpenseDate} {
		if leg != before && leg != after {
			t.Errorf("a date-less conversion leg is dated %q, want today (%s)", leg, before)
		}
	}
}
