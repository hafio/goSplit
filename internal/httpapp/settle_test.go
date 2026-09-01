package httpapp

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// tripHarness registers Alice (1), makes Bob (2) a friend and a member of group
// 1, and records a 100.00 EQUAL expense Bob paid for both — leaving Alice owing
// Bob 50.00 inside the group.
func tripHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	return h
}

// bothRegistered is like tripHarness but registers Bob (2) as a real account too,
// so the login helper can switch the session to him. Alice (1) and Bob are group
// members and Bob paid the 100.00 Hotel, leaving Alice owing Bob 50.00 in group 1.
// The session is left as Alice.
func bothRegistered(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // 1
	h.register("Bob", "bob@example.com", "password123")     // 2, session now Bob
	h.login("alice@example.com", "password123")             // back to Alice
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	return h
}

// chainHarness extends tripHarness (Alice owes Bob 50) with Carol (3) and a Taxi
// that Carol paid for herself and Bob, so Bob owes Carol 50. Net across the group
// is Alice -50, Bob 0, Carol +50 — a chain whose min-cash-flow settlement is a
// single Alice -> Carol 50, versus the two raw pairwise legs the page shows with
// simplification off.
func chainHarness(t *testing.T) *harness {
	t.Helper()
	h := tripHarness(t)
	h.post("/friends/add", url.Values{"email": {"carol@example.com"}}) // 3
	h.post("/groups/1/invite", url.Values{"email": {"carol@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Taxi"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"3"},
		"group_id": {"1"}, "include_2": {"1"}, "include_3": {"1"},
	})
	return h
}

// settlementGroups returns the group_id of every settlement row, so tests can
// assert both that a settlement exists and which bucket it landed in.
func settlementGroups(t *testing.T, h *harness) []sql.NullInt64 {
	t.Helper()
	rows, err := h.st.DB.Query(`SELECT group_id FROM expenses WHERE split_type = 'SETTLEMENT'`)
	if err != nil {
		t.Fatalf("query settlements: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []sql.NullInt64
	for rows.Next() {
		var g sql.NullInt64
		if err := rows.Scan(&g); err != nil {
			t.Fatalf("scan settlement: %v", err)
		}
		out = append(out, g)
	}
	return out
}

func assertNoSettlement(t *testing.T, h *harness) {
	t.Helper()
	if got := settlementGroups(t, h); len(got) != 0 {
		t.Errorf("%d settlement(s) recorded, want none", len(got))
	}
}

// TestGroupSettleClearsGroupBalance is the whole point of the group settle route:
// a payment recorded from the group page carries the group, so it lands in the
// same balance_view bucket as the debt and the group reads as settled.
func TestGroupSettleClearsGroupBalance(t *testing.T) {
	h := tripHarness(t)

	if b := body(t, h.get("/groups/1")); !strings.Contains(b, "/groups/1/settle/2") {
		t.Fatal("group page missing a group-scoped settle link")
	}

	_ = body(t, h.post("/groups/1/settle/2", url.Values{
		"amount": {"50.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}))

	b := body(t, h.get("/groups/1"))
	if strings.Contains(b, "/groups/1/settle/2") {
		t.Error("group still shows a debt after a group settlement")
	}
	if !strings.Contains(b, "Everyone is settled up.") {
		t.Error("group page missing the settled-up copy")
	}
	if f := body(t, h.get("/friends/2")); !strings.Contains(f, "settled up") {
		t.Error("friend balance not cleared by a group settlement")
	}

	got := settlementGroups(t, h)
	if len(got) != 1 {
		t.Fatalf("%d settlements recorded, want 1", len(got))
	}
	if !got[0].Valid || got[0].Int64 != 1 {
		t.Errorf("settlement group_id = %+v, want 1", got[0])
	}
}

// TestFriendSettleLeavesGroupBalance pins the documented asymmetry: a friend
// settlement carries no group, so it clears the cross-group net on the friend
// page but deliberately leaves each group's own balance alone.
func TestFriendSettleLeavesGroupBalance(t *testing.T) {
	h := tripHarness(t)

	_ = body(t, h.post("/friends/2/settle", url.Values{
		"amount": {"50.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}))

	if f := body(t, h.get("/friends/2")); !strings.Contains(f, "settled up") {
		t.Error("friend balance not cleared by a friend settlement")
	}
	if g := body(t, h.get("/groups/1")); !strings.Contains(g, "/groups/1/settle/2") {
		t.Error("group balance unexpectedly cleared by a direct settlement")
	}

	got := settlementGroups(t, h)
	if len(got) != 1 {
		t.Fatalf("%d settlements recorded, want 1", len(got))
	}
	if got[0].Valid {
		t.Errorf("friend settlement should carry no group, got %+v", got[0])
	}
}

// TestGroupSettlePagePrefill checks the form is seeded from the group's own debt
// (not the cross-group friend net) and that an unmatched ?cur= is dropped rather
// than echoed back into the page.
func TestGroupSettlePagePrefill(t *testing.T) {
	h := tripHarness(t)

	b := body(t, h.get("/groups/1/settle/2?cur=USD"))
	if !strings.Contains(b, `value="50.00"`) {
		t.Error("settle form not prefilled with the group debt")
	}
	if !strings.Contains(b, `action="/groups/1/settle/2"`) {
		t.Error("settle form does not post to the group settle route")
	}
	if !strings.Contains(b, `href="/groups/1"`) {
		t.Error("cancel link should return to the group")
	}

	if b := body(t, h.get("/groups/1/settle/2?cur=ZZZ")); strings.Contains(b, "ZZZ") {
		t.Error("unmatched cur query echoed into the form")
	}
}

// TestGroupSettleRejectsNonMember covers the recipient-side authorization
// boundary: a user outside the group can never be paid through it.
func TestGroupSettleRejectsNonMember(t *testing.T) {
	h := tripHarness(t)
	h.post("/friends/add", url.Values{"email": {"carol@example.com"}}) // id 3, not in the group

	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/groups/1/settle/3", url.Values{
		"amount": {"10.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST status %d, want 400", resp.StatusCode)
	}
	if b := body(t, resp); !strings.Contains(b, "not a member of this group") {
		t.Error("missing err.settle_not_member message")
	}
	assertNoSettlement(t, h)
}

// TestGroupExpenseRejectsNonMember covers the AddExpense membership guard: a
// forged post naming a participant who is not a group member is refused and no
// expense is written (the add form only ever offers members, so this can only be
// hit by a hand-crafted request).
func TestGroupExpenseRejectsNonMember(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // 2, never invited
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})

	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST status %d, want 400", resp.StatusCode)
	}
	if b := body(t, resp); !strings.Contains(b, "must be members of the target group") {
		t.Error("missing the non-member rejection message")
	}
	var n int
	if err := h.st.DB.QueryRow(`SELECT COUNT(*) FROM expenses WHERE name = 'Hotel'`).Scan(&n); err != nil {
		t.Fatalf("count expenses: %v", err)
	}
	if n != 0 {
		t.Errorf("expense created despite a non-member participant (%d rows)", n)
	}
}

// TestGroupSettleFromCreditor exercises the both-ways direction: Bob, the creditor,
// records the debt Alice owes him. Direction is derived server-side from the group
// balance, so the recorded settlement still has Alice (the debtor) as the payer.
func TestGroupSettleFromCreditor(t *testing.T) {
	h := bothRegistered(t) // Alice owes Bob 50 in group 1
	h.login("bob@example.com", "password123")

	if b := body(t, h.get("/groups/1/settle/1")); !strings.Contains(b, `value="50.00"`) {
		t.Error("creditor's settle form not prefilled with the debt owed to them")
	}
	_ = body(t, h.post("/groups/1/settle/1", url.Values{
		"amount": {"50.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}))

	if b := body(t, h.get("/groups/1")); !strings.Contains(b, "Everyone is settled up.") {
		t.Error("group not settled after the creditor recorded the payment")
	}
	var paidBy int64
	var gid sql.NullInt64
	if err := h.st.DB.QueryRow(
		`SELECT paid_by, group_id FROM expenses WHERE split_type = 'SETTLEMENT'`,
	).Scan(&paidBy, &gid); err != nil {
		t.Fatalf("query settlement: %v", err)
	}
	if paidBy != 1 {
		t.Errorf("settlement paid_by = %d, want 1 (Alice, the debtor)", paidBy)
	}
	if !gid.Valid || gid.Int64 != 1 {
		t.Errorf("settlement group_id = %+v, want 1", gid)
	}
}

// TestFriendSettleFromCreditor is the friend-page counterpart: Alice records a
// direct debt Bob owes her. The payer on the settlement is Bob, derived from the
// balance rather than taken from the form.
func TestFriendSettleFromCreditor(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // 2
	// Alice pays a direct 40.00 split with Bob -> Bob owes Alice 20.
	h.post("/expenses", url.Values{
		"name": {"Lunch"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})

	if b := body(t, h.get("/friends/2/settle")); !strings.Contains(b, `value="20.00"`) {
		t.Error("friend settle form not prefilled with the debt owed to me")
	}
	_ = body(t, h.post("/friends/2/settle", url.Values{
		"amount": {"20.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}))

	if f := body(t, h.get("/friends/2")); !strings.Contains(f, "settled up") {
		t.Error("friend balance not cleared after recording the debt owed to me")
	}
	var paidBy int64
	if err := h.st.DB.QueryRow(
		`SELECT paid_by FROM expenses WHERE split_type = 'SETTLEMENT'`,
	).Scan(&paidBy); err != nil {
		t.Fatalf("query settlement: %v", err)
	}
	if paidBy != 2 {
		t.Errorf("settlement paid_by = %d, want 2 (Bob, the debtor)", paidBy)
	}
}

// TestGroupSettleAllMinimizesAndZeroes checks that settling the whole group records
// the minimum number of transfers (min-cash-flow) — one Alice -> Carol 50, not the
// two raw pairwise legs — nets every member to zero, and is idempotent. The second
// sub-case toggles the display Simplify flag off first to prove settle-group ignores
// it and still records the minimal set.
func TestGroupSettleAllMinimizesAndZeroes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		simplifyOff bool
	}{
		{"simplify on (default)", false},
		{"simplify off — settle-group ignores the display toggle", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := chainHarness(t)
			if tc.simplifyOff {
				_ = body(t, h.post("/groups/1/simplify", url.Values{}))
			}

			if b := body(t, h.get("/groups/1/settle")); !strings.Contains(b, "50.00") {
				t.Error("settle-group confirmation missing the minimal transfer")
			}

			_ = body(t, h.post("/groups/1/settle", url.Values{"date": {"2026-03-12"}}))
			if got := settlementGroups(t, h); len(got) != 1 {
				t.Fatalf("%d settlements recorded, want exactly 1 (min-cash-flow set)", len(got))
			}

			// Every member now nets to zero, so the whole-group confirmation reports
			// nothing outstanding — regardless of the display toggle.
			if b := body(t, h.get("/groups/1/settle")); !strings.Contains(b, "already settled up") {
				t.Error("settle-group still lists transfers after the group nets to zero")
			}

			// Idempotent: a second confirm recomputes an empty set and records nothing.
			_ = body(t, h.post("/groups/1/settle", url.Values{"date": {"2026-03-13"}}))
			if got := settlementGroups(t, h); len(got) != 1 {
				t.Errorf("second settle-group added rows (%d total), want still 1", len(got))
			}
		})
	}
}

// TestGroupSettleAllRecordsInterMemberTransfer proves the "entire group" behaviour:
// the acting member (Alice) can settle a debt strictly between two other members.
// Bob owes Carol; Alice holds no balance yet records the single Bob -> Carol
// transfer, with herself as added_by.
func TestGroupSettleAllRecordsInterMemberTransfer(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // 1, owner, not a participant
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})   // 2
	h.post("/friends/add", url.Values{"email": {"carol@example.com"}}) // 3
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/1/invite", url.Values{"email": {"carol@example.com"}})
	// Carol pays; only Bob and Carol share -> Bob owes Carol 50. Alice is uninvolved.
	h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"3"},
		"group_id": {"1"}, "include_2": {"1"}, "include_3": {"1"},
	})

	_ = body(t, h.post("/groups/1/settle", url.Values{"date": {"2026-03-12"}}))

	var count, paidBy, addedBy int64
	if err := h.st.DB.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(paid_by), 0), COALESCE(MAX(added_by), 0) FROM expenses WHERE split_type = 'SETTLEMENT'`,
	).Scan(&count, &paidBy, &addedBy); err != nil {
		t.Fatalf("query settlement: %v", err)
	}
	if count != 1 {
		t.Fatalf("%d settlements recorded, want 1 (the Bob -> Carol transfer)", count)
	}
	if paidBy != 2 {
		t.Errorf("settlement paid_by = %d, want 2 (Bob, the debtor)", paidBy)
	}
	if addedBy != 1 {
		t.Errorf("settlement added_by = %d, want 1 (Alice, the acting member)", addedBy)
	}
}

// TestSettlementReversedByDelete is the reversal regression: deleting a settlement's
// expense removes it from balance_view, so the original debt reappears on both the
// group and friend pages. No dedicated "unsettle" flow is needed.
func TestSettlementReversedByDelete(t *testing.T) {
	h := tripHarness(t) // Alice owes Bob 50 in group 1

	_ = body(t, h.post("/groups/1/settle/2", url.Values{
		"amount": {"50.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}))
	if b := body(t, h.get("/groups/1")); !strings.Contains(b, "Everyone is settled up.") {
		t.Fatal("group not settled after the settlement")
	}

	var id string
	if err := h.st.DB.QueryRow(
		`SELECT id FROM expenses WHERE split_type = 'SETTLEMENT'`,
	).Scan(&id); err != nil {
		t.Fatalf("look up settlement id: %v", err)
	}
	_ = body(t, h.post("/expenses/"+id+"/delete", url.Values{"version": {"1"}}))

	if b := body(t, h.get("/groups/1")); !strings.Contains(b, "/groups/1/settle/2") {
		t.Error("group debt not restored after deleting the settlement")
	}
	if f := body(t, h.get("/friends/2")); strings.Contains(f, "settled up") {
		t.Error("friend balance still reads settled after deleting the settlement")
	}
}

// TestExpenseDeleteRejectsNonEditor pins the tightened delete authorization: a group
// member who is not part of the transaction cannot delete it. Carol is a fresh
// account (not a member, not a participant); her delete is refused and the expense
// stays live.
func TestExpenseDeleteRejectsNonEditor(t *testing.T) {
	h := tripHarness(t) // Hotel: Bob paid, participants Alice + Bob
	h.register("Carol", "carol@example.com", "password123") // 3, session switches to Carol

	var id string
	if err := h.st.DB.QueryRow(`SELECT id FROM expenses WHERE name = 'Hotel'`).Scan(&id); err != nil {
		t.Fatalf("look up expense id: %v", err)
	}

	resp := h.post("/expenses/"+id+"/delete", url.Values{})
	if b := body(t, resp); resp.StatusCode != http.StatusForbidden {
		t.Errorf("delete by a non-editor: status %d, want 403 (body: %s)", resp.StatusCode, b)
	}

	var deletedAt sql.NullString
	if err := h.st.DB.QueryRow(`SELECT deleted_at FROM expenses WHERE id = ?`, id).Scan(&deletedAt); err != nil {
		t.Fatalf("re-read expense: %v", err)
	}
	if deletedAt.Valid {
		t.Error("expense was soft-deleted by a non-editor")
	}
}

// TestGroupSettleRejectsOutsider covers the caller-side boundary: a user who is
// not in the group cannot reach its settle form or post to it.
func TestGroupSettleRejectsOutsider(t *testing.T) {
	h := tripHarness(t)
	h.register("Carol", "carol@example.com", "password123") // id 3, session switches to Carol
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	if resp := h.get("/groups/1/settle/2"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET status %d, want 403", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	if resp := h.post("/groups/1/settle/2", url.Values{
		"amount": {"10.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
	}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST status %d, want 403", resp.StatusCode)
		_ = body(t, resp)
	} else {
		_ = body(t, resp)
	}
	assertNoSettlement(t, h)
}

// TestSettleRejectsInvalidAmounts covers the shared amount guard on both routes:
// a non-positive or unparseable amount is refused and nothing is written.
func TestSettleRejectsInvalidAmounts(t *testing.T) {
	h := tripHarness(t)
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	for _, path := range []string{"/groups/1/settle/2", "/friends/2/settle"} {
		for _, amt := range []string{"0", "0.00", "-5.00", "abc", ""} {
			resp := h.post(path, url.Values{"amount": {amt}, "currency": {"USD"}, "date": {"2026-03-11"}})
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("POST %s amount %q: status %d, want 400", path, amt, resp.StatusCode)
			}
			if b := body(t, resp); !strings.Contains(b, "positive settlement amount") {
				t.Errorf("POST %s amount %q: missing invalid-amount message", path, amt)
			}
		}
	}
	assertNoSettlement(t, h)
}

// TestGroupSimplifyTogglesView proves the flag now gates simplification rather
// than only relabelling the heading. Alice owes Bob 50 and Bob owes Carol 50, so
// simplification collapses the chain to a single Alice → Carol transfer while the
// raw view keeps both pairwise debts.
func TestGroupSimplifyTogglesView(t *testing.T) {
	h := tripHarness(t)
	h.post("/friends/add", url.Values{"email": {"carol@example.com"}}) // id 3
	h.post("/groups/1/invite", url.Values{"email": {"carol@example.com"}})
	h.post("/expenses", url.Values{
		"name": {"Taxi"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"3"},
		"group_id": {"1"}, "include_2": {"1"}, "include_3": {"1"},
	})

	// Simplification is on for new groups: the chain collapses to Alice → Carol.
	b := body(t, h.get("/groups/1"))
	if !strings.Contains(b, "Simplified settlements") {
		t.Error("simplified heading missing while the flag is on")
	}
	if !strings.Contains(b, "/groups/1/settle/3") {
		t.Error("simplified view should route Alice's payment to Carol")
	}
	if strings.Contains(b, "/groups/1/settle/2") {
		t.Error("simplified view should not keep the Alice → Bob leg")
	}

	_ = body(t, h.post("/groups/1/simplify", url.Values{}))

	// Off: raw pairwise debts, so Alice pays Bob and Bob pays Carol.
	b = body(t, h.get("/groups/1"))
	if strings.Contains(b, "Simplified settlements") {
		t.Error("simplified heading still shown while the flag is off")
	}
	if !strings.Contains(b, "/groups/1/settle/2") {
		t.Error("raw view should keep the Alice → Bob debt")
	}
	if strings.Contains(b, "/groups/1/settle/3") {
		t.Error("raw view should not offer Alice a payment to Carol")
	}
}
