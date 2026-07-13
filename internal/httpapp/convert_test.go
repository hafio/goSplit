package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRateEndpoint(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	// Happy path: seed the cache so no provider call is needed.
	_ = h.st.PutCachedRate(context.Background(), "SGD", "USD", "latest", "0.78")
	if b := body(t, h.get("/rates?from=SGD&to=USD")); !strings.Contains(b, `"rate":"0.78"`) {
		t.Errorf("rate endpoint body = %s", b)
	}

	// Validation gate: bad currency / bad date → 400, no provider call.
	for _, q := range []string{"/rates?from=SG&to=USD", "/rates?from=SGD&to=US1", "/rates?from=SGD&to=USD&date=2026/01/01"} {
		resp := h.get(q)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, resp.StatusCode)
		}
	}
}

// TestConvertExactBothAmounts converts a balance the friend owes you (SGD) into
// USD using explicit amounts, and confirms both legs store those amounts and the
// balance moves currency.
func TestConvertExactBothAmounts(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	// Bob owes Alice 100.00 SGD (Alice paid a 200.00 SGD equal split).
	h.post("/expenses", url.Values{
		"name": {"Hotpot"}, "amount": {"200.00"}, "currency": {"SGD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})

	// Convert 100.00 SGD → 78.00 USD (friend owes you).
	resp := h.post("/friends/2/convert", url.Values{
		"direction": {"owed"}, "from_currency": {"SGD"}, "to_currency": {"USD"},
		"from_amount": {"100.00"}, "to_amount": {"78.00"}, "date": {"2026-01-03"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("convert status %d", resp.StatusCode)
	}
	_ = body(t, resp)

	// Both conversion legs stored with the exact submitted amounts.
	var sgd, usd int64
	_ = h.st.DB.QueryRow(`SELECT amount FROM expenses WHERE split_type='CURRENCY_CONVERSION' AND currency='SGD'`).Scan(&sgd)
	_ = h.st.DB.QueryRow(`SELECT amount FROM expenses WHERE split_type='CURRENCY_CONVERSION' AND currency='USD'`).Scan(&usd)
	if sgd != 10000 || usd != 7800 {
		t.Fatalf("legs stored %d SGD / %d USD, want 10000 / 7800", sgd, usd)
	}

	// The balance MOVED currency: SGD nets to zero, USD now carries it.
	var sgdBal, usdBal int64
	_ = h.st.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM balance_view WHERE user_id=1 AND friend_id=2 AND currency='SGD'`).Scan(&sgdBal)
	_ = h.st.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM balance_view WHERE user_id=1 AND friend_id=2 AND currency='USD'`).Scan(&usdBal)
	if sgdBal != 0 {
		t.Errorf("SGD balance not cancelled by conversion: %d", sgdBal)
	}
	if usdBal != 7800 {
		t.Errorf("USD balance = %d, want 7800 (bob owes alice 78.00 USD)", usdBal)
	}
}
