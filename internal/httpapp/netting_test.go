package httpapp

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// settledChainHarness is chainHarness (Alice owes Bob 50, Bob owes Carol 50 in
// group 1) after "Settle group": one Alice -> Carol payment plus the three
// netting entries that clear the pairs. The session is Alice's.
func settledChainHarness(t *testing.T) *harness {
	t.Helper()
	h := chainHarness(t)
	_ = body(t, h.post("/groups/1/settle", url.Values{"date": {"2026-03-12"}}))
	return h
}

// nettingBetween returns the netting entry between users x and y.
func nettingBetween(t *testing.T, h *harness, x, y int64) *store.Expense {
	t.Helper()
	var id string
	err := h.st.DB.QueryRow(h.st.Rebind(`SELECT e.id FROM expenses e WHERE e.split_type = 'NETTING'
		AND EXISTS (SELECT 1 FROM expense_participants p WHERE p.expense_id = e.id AND p.user_id = ?)
		AND EXISTS (SELECT 1 FROM expense_participants p WHERE p.expense_id = e.id AND p.user_id = ?)`), x, y).Scan(&id)
	if err != nil {
		t.Fatalf("netting entry between %d and %d: %v", x, y, err)
	}
	e, err := h.st.GetExpense(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestNettingRowsRenderReadOnly: netting entries show in the group feed and in
// the pair's friend history under their own label, never as "you lent", and
// their detail page explains them and offers no edit or delete.
func TestNettingRowsRenderReadOnly(t *testing.T) {
	h := settledChainHarness(t)
	group := body(t, h.get("/groups/1"))
	for _, want := range []string{"Group debts simplified", "Automatic entry"} {
		if !strings.Contains(group, want) {
			t.Errorf("group feed missing %q", want)
		}
	}
	friend := body(t, h.get("/friends/2"))
	if !strings.Contains(friend, "Group debts simplified") {
		t.Error("Alice's history with Bob does not show their netting entry")
	}

	entry := nettingBetween(t, h, 1, 2)
	detail := body(t, h.get("/expenses/" + entry.ID))
	if !strings.Contains(detail, "Recorded automatically when this group settled up") {
		t.Error("netting detail page missing its explanation")
	}
	for _, control := range []string{"/expenses/" + entry.ID + "/move", "/expenses/" + entry.ID + "/delete"} {
		if strings.Contains(detail, control) {
			t.Errorf("netting detail page offers %q", control)
		}
	}
}

// TestNettingEntryRefusesChanges: deleting or editing a netting entry by hand --
// even as one of its two people -- is refused with the explanation, and the
// entry is untouched.
func TestNettingEntryRefusesChanges(t *testing.T) {
	h := settledChainHarness(t)
	entry := nettingBetween(t, h, 1, 2)
	covNoRedirect(h)
	const msg = "This is an automatic entry that keeps the group"

	resp := h.post("/expenses/"+entry.ID+"/delete", url.Values{"version": {fmt.Sprint(entry.Version)}})
	if b := body(t, resp); resp.StatusCode != http.StatusForbidden || !strings.Contains(b, msg) {
		t.Errorf("delete of a netting entry = %d, want 403 with the explanation", resp.StatusCode)
	}
	resp = h.get("/expenses/" + entry.ID + "/move")
	if b := body(t, resp); resp.StatusCode != http.StatusForbidden || !strings.Contains(b, msg) {
		t.Errorf("edit form of a netting entry = %d, want 403 with the explanation", resp.StatusCode)
	}
	resp = h.post("/expenses/"+entry.ID+"/move", url.Values{
		"name": {"Mine now"}, "amount": {"50.00"}, "currency": {"USD"}, "method": {"EQUAL"},
		"paid_by": {"1"}, "include_1": {"1"}, "include_2": {"1"}, "target_group": {"1"},
		"version": {fmt.Sprint(entry.Version)},
	})
	if b := body(t, resp); resp.StatusCode != http.StatusForbidden || !strings.Contains(b, msg) {
		t.Errorf("forged edit of a netting entry = %d, want 403 with the explanation", resp.StatusCode)
	}

	after, _ := h.st.GetExpense(context.Background(), entry.ID)
	if after.SplitType != "NETTING" || after.Version != entry.Version || after.DeletedAt.Valid || after.Name != entry.Name {
		t.Errorf("a refused change still touched the netting entry: %+v", after)
	}
}

// TestTidyUpClearsLeftoverPairs: a group settled before netting existed (here:
// the Alice -> Carol payment written straight to the store, so no netting ran)
// reads settled on its own page but not pairwise; its members get a Tidy up
// action that clears the pairs, outsiders cannot use it, and afterwards the
// action is gone.
func TestTidyUpClearsLeftoverPairs(t *testing.T) {
	h := chainHarness(t)
	gid := int64(1)
	if _, err := h.st.CreateExpense(context.Background(), &store.Expense{
		Name: "Settlement", Category: "settlement", Amount: 5000, SplitType: "SETTLEMENT",
		ExpenseDate: "2026-03-12", Currency: "USD", PaidBy: 1, AddedBy: 1, GroupID: sql.NullInt64{Int64: gid, Valid: true},
	}, []store.ExpenseParticipant{{UserID: 1, Amount: 5000}, {UserID: 3, Amount: -5000}}); err != nil {
		t.Fatal(err)
	}
	page := body(t, h.get("/groups/1"))
	if !strings.Contains(page, `action="/groups/1/tidy"`) || !strings.Contains(page, "Tidy up balances") {
		t.Fatal("settled group with leftover pairs does not offer the tidy-up")
	}

	h.register("Dave", "dave@example.com", "password123") // 4, not in the group
	covNoRedirect(h)
	resp := h.post("/groups/1/tidy", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("outsider tidy = %d, want 403", resp.StatusCode)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM expenses WHERE split_type = 'NETTING'`); n != 0 {
		t.Fatalf("%d netting entries after a refused tidy-up", n)
	}

	h.client.CheckRedirect = nil
	h.login("alice@example.com", "password123")
	covNoRedirect(h)
	resp = h.post("/groups/1/tidy", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("member tidy = %d, want 303", resp.StatusCode)
	}
	if bals, _ := h.st.GroupBalances(context.Background(), 1); len(bals) != 0 {
		t.Errorf("pairs after tidying = %v, want none", bals)
	}
	h.client.CheckRedirect = nil
	if page := body(t, h.get("/groups/1")); strings.Contains(page, `action="/groups/1/tidy"`) {
		t.Error("tidy-up still offered after the pairs are clear")
	}
}
