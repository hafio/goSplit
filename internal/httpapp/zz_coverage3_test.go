package httpapp

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// seedBankTx stores a single cached bank transaction for the given user by
// writing the service's bankData JSON directly through the store. This reaches
// the cached-transaction branches (bank page row loop, convert redirect) without
// an enabled bank.Provider, which the harness cannot inject.
func covSeedBankTx(h *harness, userID int64) {
	h.t.Helper()
	const data = `{"accessToken":"","transactions":[{"id":"tx-coffee-1","name":"Coffee","amountMinor":450,"currency":"USD","date":"2026-01-15"}],"syncedAt":"2026-01-15T00:00:00Z"}`
	if err := h.st.PutBankData(context.Background(), userID, data); err != nil {
		h.t.Fatalf("seed bank data: %v", err)
	}
}

// TestCov3BankPageWithCachedTx covers handleBankPage's row-building loop (the
// `for _, t := range txns` branch), which the round-1 test left untested because
// no cached transactions existed. The page still renders 200 even with the
// provider disabled.
func TestCov3BankPageWithCachedTx(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // id 1
	covSeedBankTx(h, 1)

	resp := h.get("/bank")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /bank with cached tx status %d, want 200", resp.StatusCode)
	}
}

// TestCov3BankConvertRedirect covers handleBankConvert's success path: a cached
// transaction is found and the handler redirects into the prefilled add-expense
// form. Only the 404 branch was covered before.
func TestCov3BankConvertRedirect(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // id 1
	covSeedBankTx(h, 1)
	covNoRedirect(h)

	resp := h.get("/bank/tx/tx-coffee-1/convert")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("bank convert status %d, want 303", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/expenses/new") {
		t.Errorf("convert redirect = %q, want /expenses/new prefix", loc)
	}
	if !strings.Contains(loc, "name=Coffee") {
		t.Errorf("convert redirect = %q, want prefilled name=Coffee", loc)
	}
	if !strings.Contains(loc, "currency=USD") {
		t.Errorf("convert redirect = %q, want prefilled currency=USD", loc)
	}
}
