package httpapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// settlementPath records a settlement in group 1 and returns its detail path.
// Alice owes Bob 50.00 there, so settling clears the group balance.
func settlementPath(t *testing.T, h *harness) string {
	t.Helper()
	h.post("/groups/1/settle/2", url.Values{
		"amount": {"50.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	})
	var id string
	if err := h.st.DB.QueryRow(
		`SELECT id FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&id); err != nil {
		t.Fatalf("no settlement recorded: %v", err)
	}
	return "/expenses/" + id
}

// TestEditSettlementPageHasNoMethodPicker: a settlement is not a split, so the
// edit form drops the method segment and the participant list and shows the pair
// it moves money between.
func TestEditSettlementPageHasNoMethodPicker(t *testing.T) {
	// bothRegistered rather than tripHarness: same balance, but Bob has a real
	// account, so the form shows names instead of the invite email fallback.
	h := bothRegistered(t)
	path := settlementPath(t, h)

	b := body(t, h.get(path+"/move"))
	if strings.Contains(b, `name="method"`) {
		t.Errorf("settlement edit form still renders a split-method picker:\n%s", b)
	}
	if strings.Contains(b, `name="include_`) {
		t.Error("settlement edit form still renders the participant checkboxes")
	}
	// The read-only pair block replaces the participant list.
	if !strings.Contains(b, "Paid to") {
		t.Errorf("settlement edit form does not show the transfer direction:\n%s", b)
	}
	if !strings.Contains(b, "Alice") || !strings.Contains(b, "Bob") {
		t.Errorf("settlement edit form does not name both sides of the transfer:\n%s", b)
	}
	if !strings.Contains(b, `name="amount"`) {
		t.Error("settlement edit form should let the amount be corrected")
	}
}

// TestEditSettlementUpdatesAmount: the whole point -- a settlement edit saves,
// stays a settlement, and moves the balance by the difference.
func TestEditSettlementUpdatesAmount(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)

	resp := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "note": {"partial"}, "version": {"1"},
	})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settlement edit status %d", resp.StatusCode)
	}
	var splitType, note string
	var amount int64
	if err := h.st.DB.QueryRow(
		`SELECT split_type, amount, note FROM expenses WHERE id = ?`,
		strings.TrimPrefix(path, "/expenses/")).Scan(&splitType, &amount, &note); err != nil {
		t.Fatalf("reload settlement: %v", err)
	}
	if splitType != "SETTLEMENT" {
		t.Errorf("split_type = %q, want SETTLEMENT (the edit converted it to a plain expense)", splitType)
	}
	if amount != 3000 || note != "partial" {
		t.Errorf("amount/note = %d/%q, want 3000/partial", amount, note)
	}
	// The two rows still mirror each other at the new amount.
	rows, err := h.st.DB.Query(
		`SELECT amount FROM expense_participants WHERE expense_id = ? ORDER BY amount`,
		strings.TrimPrefix(path, "/expenses/"))
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	defer rows.Close()
	var got []int64
	for rows.Next() {
		var a int64
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, a)
	}
	if len(got) != 2 || got[0] != -3000 || got[1] != 3000 {
		t.Errorf("participant rows = %v, want [-3000 3000]", got)
	}
}

// TestEditSettlementRejectsBadAmount: a bad amount comes back on the form with
// the message attached, not as a dead-end page.
func TestEditSettlementRejectsBadAmount(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)

	resp := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "amount": {"nonsense"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "version": {"1"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(b, "expense-form") {
		t.Errorf("rejected settlement edit did not re-render the form:\n%s", b)
	}
	if !strings.Contains(b, "field-err") {
		t.Error("no field-level error rendered for the bad amount")
	}
	var amount int64
	_ = h.st.DB.QueryRow(`SELECT amount FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&amount)
	if amount != 5000 {
		t.Errorf("amount = %d, want the original 5000 (nothing should persist on a rejected edit)", amount)
	}
}

// TestEditSettlementGroupMoveNeedsAck: relocating a settlement changes which
// balance it clears, so it takes the same acknowledgment as any other move.
func TestEditSettlementGroupMoveNeedsAck(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)

	noAck := h.post(path+"/move", url.Values{
		"target_group": {"none"}, "amount": {"50.00"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "version": {"1"},
	})
	_ = body(t, noAck)
	if noAck.StatusCode != http.StatusBadRequest {
		t.Fatalf("group change without ack: status %d, want 400", noAck.StatusCode)
	}
	withAck := h.post(path+"/move", url.Values{
		"target_group": {"none"}, "amount": {"50.00"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "ack": {"1"}, "version": {"1"},
	})
	_ = body(t, withAck)
	if withAck.StatusCode != http.StatusOK {
		t.Fatalf("group change with ack: status %d", withAck.StatusCode)
	}
	var gid interface{}
	_ = h.st.DB.QueryRow(`SELECT group_id FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&gid)
	if gid != nil {
		t.Errorf("settlement group = %v, want NULL after moving it out of the group", gid)
	}
}

// TestEditSettlementHidesFixedFieldPickers: a settlement's currency is fixed for
// the life of the record and its category is never read back, so neither is
// offered as a picker -- the currency shows as text beside the amount.
func TestEditSettlementHidesFixedFieldPickers(t *testing.T) {
	h := bothRegistered(t)
	path := settlementPath(t, h)

	b := body(t, h.get(path+"/move"))
	if strings.Contains(b, `name="currency"`) {
		t.Errorf("settlement edit form still offers a currency picker:\n%s", b)
	}
	if strings.Contains(b, `name="category"`) {
		t.Error("settlement edit form still offers a category picker, whose value is discarded")
	}
	if !strings.Contains(b, `class="pill static cur-btn"`) {
		t.Errorf("the fixed currency is not shown beside the amount:\n%s", b)
	}
}

// TestEditSettlementKeepsZeroDecimalCurrency: with no currency on the form, the
// edit is read in the settlement's own currency. Falling back to a hardcoded USD
// misread a zero-decimal amount by a factor of 100 and flipped the row's
// currency with it.
func TestEditSettlementKeepsZeroDecimalCurrency(t *testing.T) {
	h := tripHarness(t)
	h.post("/groups/1/settle/2", url.Values{
		"amount": {"1000"}, "currency": {"JPY"}, "date": {"2026-03-11"},
	})
	var id string
	if err := h.st.DB.QueryRow(
		`SELECT id FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&id); err != nil {
		t.Fatalf("no settlement recorded: %v", err)
	}

	resp := h.post("/expenses/"+id+"/move", url.Values{
		"target_group": {"1"}, "amount": {"1500"}, "date": {"2026-03-11"}, "version": {"1"},
	})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settlement edit status %d", resp.StatusCode)
	}
	var amount int64
	var currency string
	if err := h.st.DB.QueryRow(
		`SELECT amount, currency FROM expenses WHERE id = ?`, id).Scan(&amount, &currency); err != nil {
		t.Fatalf("reload settlement: %v", err)
	}
	if currency != "JPY" {
		t.Errorf("currency = %q, want JPY (the edit re-denominated the settlement)", currency)
	}
	if amount != 1500 {
		t.Errorf("amount = %d, want 1500 minor units (USD parsing would give 150000)", amount)
	}
}

// TestEditSettlementRejectsCurrencySwitch: the picker is gone from the form, so
// a currency only arrives from a hand-built post. Balances bucket by currency,
// so accepting one would leave the cleared debt outstanding and invent an
// offsetting balance in the new currency.
func TestEditSettlementRejectsCurrencySwitch(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)

	resp := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "amount": {"50.00"}, "currency": {"EUR"},
		"date": {"2026-03-11"}, "version": {"1"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(b, "field-err") {
		t.Errorf("the refusal did not render against a field:\n%s", b)
	}
	var currency string
	_ = h.st.DB.QueryRow(`SELECT currency FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&currency)
	if currency != "USD" {
		t.Errorf("currency = %q, want the original USD", currency)
	}
}

// TestEditSettlementRejectsZeroAmount: a non-positive amount lands on the amount
// box like the sibling parse failure does, not as a banner on its own.
func TestEditSettlementRejectsZeroAmount(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)

	resp := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "amount": {"0.00"}, "date": {"2026-03-11"}, "version": {"1"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(b, "field-err") {
		t.Errorf("no field-level error rendered for the zero amount:\n%s", b)
	}
	var amount int64
	_ = h.st.DB.QueryRow(`SELECT amount FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&amount)
	if amount != 5000 {
		t.Errorf("amount = %d, want the original 5000", amount)
	}
}

// TestEditSettlementNonMemberRendersFieldError covers both halves of the target-
// group guard from the outside: moving a settlement into a group the
// counterparty does not belong to is refused, and the refusal renders under the
// form rather than as a banner only -- the participants error block used to sit
// inside the non-settlement branch, so a settlement never showed it.
func TestEditSettlementNonMemberRendersFieldError(t *testing.T) {
	h := tripHarness(t)
	path := settlementPath(t, h)
	// Group 2 has Alice (its creator) but not Bob.
	h.post("/groups/create", url.Values{"name": {"Solo"}, "currency": {"USD"}})

	resp := h.post(path+"/move", url.Values{
		"target_group": {"2"}, "amount": {"50.00"}, "date": {"2026-03-11"},
		"ack": {"1"}, "version": {"1"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(b, "field-err") {
		t.Errorf("membership refusal rendered as a banner only:\n%s", b)
	}
	if !strings.Contains(b, "members of the target group") {
		t.Errorf("the refusal does not say what is wrong:\n%s", b)
	}
	var gid int64
	_ = h.st.DB.QueryRow(`SELECT group_id FROM expenses WHERE split_type = 'SETTLEMENT'`).Scan(&gid)
	if gid != 1 {
		t.Errorf("settlement group = %d, want it left in group 1", gid)
	}
}
