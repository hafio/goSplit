package httpapp

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// TestFriendPagesNeedARelationship: the friend page and its settle, convert and
// collapse pages answer 404 for an account the caller shares nothing with, and
// ?friend=<id> on the add form does not list it -- ids are sequential, so
// otherwise any account's name or email could be read. Becoming friends opens
// them (control).
func TestFriendPagesNeedARelationship(t *testing.T) {
	h := newHarness(t)
	h.register("Carol", "carol@example.com", "password123") // 1
	h.register("Alice", "alice@example.com", "password123") // 2, signed in

	for _, p := range []string{"/friends/1", "/friends/1/settle", "/friends/1/convert", "/friends/1/collapse"} {
		resp := h.get(p)
		if b := body(t, resp); resp.StatusCode != http.StatusNotFound || strings.Contains(b, "carol@example.com") || strings.Contains(b, "Carol") {
			t.Errorf("GET %s for a stranger = %d, want 404 without her name", p, resp.StatusCode)
		}
	}
	if b := body(t, h.get("/expenses/new?friend=1")); strings.Contains(b, "carol@example.com") || strings.Contains(b, "Carol") {
		t.Error("the add form listed a stranger picked by id")
	}

	_ = body(t, h.post("/friends/add", url.Values{"email": {"carol@example.com"}}))
	if resp := h.get("/friends/1"); resp.StatusCode != http.StatusOK {
		t.Errorf("friend page after befriending = %d, want 200", resp.StatusCode)
	} else if b := body(t, resp); !strings.Contains(b, "Carol") {
		t.Error("the friend page does not name the friend")
	}
}

// TestDirectMoneyWithAStrangerIsRefused: an expense, a settlement or a
// conversion aimed at an account the caller shares nothing with -- by posting
// its id -- is refused, and nothing is recorded or befriended.
func TestDirectMoneyWithAStrangerIsRefused(t *testing.T) {
	h := newHarness(t)
	h.register("Carol", "carol@example.com", "password123") // 1
	h.register("Alice", "alice@example.com", "password123") // 2, signed in

	resp := h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"}, "date": {"2026-01-02"},
		"method": {"EQUAL"}, "paid_by": {"2"}, "include_1": {"1"}, "include_2": {"1"},
	})
	if b := body(t, resp); resp.StatusCode != http.StatusBadRequest || !strings.Contains(b, "friends or people in one of your groups") {
		t.Errorf("expense with a stranger = %d, want 400 explaining why", resp.StatusCode)
	}
	resp = h.post("/friends/1/settle", url.Values{"amount": {"5.00"}, "currency": {"USD"}, "date": {"2026-01-02"}})
	if _ = body(t, resp); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("settlement with a stranger = %d, want 400", resp.StatusCode)
	}
	resp = h.post("/friends/1/convert", url.Values{
		"direction": {"owed"}, "from_currency": {"SGD"}, "to_currency": {"USD"},
		"from_amount": {"100.00"}, "to_amount": {"78.00"}, "date": {"2026-01-03"},
	})
	if _ = body(t, resp); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("conversion with a stranger = %d, want 400", resp.StatusCode)
	}

	if n := countOf(t, h, `SELECT COUNT(*) FROM expenses`); n != 0 {
		t.Errorf("%d expenses recorded with a stranger", n)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM friendships`); n != 0 {
		t.Errorf("%d friendship rows after refused writes", n)
	}
}

// TestExpenseDetailIsForThoseInvolved: an expense's page opens for its
// participants and for members of its group, and is a 404 -- like an unknown
// id -- for anyone else.
func TestExpenseDetailIsForThoseInvolved(t *testing.T) {
	h := newHarness(t)
	for _, n := range []string{"Alice", "Bob", "Dave", "Erin", "Carol"} { // 1..5; Carol ends signed in
		h.register(n, strings.ToLower(n)+"@example.com", "password123")
	}
	ctx := context.Background()
	direct, err := h.st.CreateExpense(ctx, &store.Expense{
		Name: "Secret dinner", Category: "general", Amount: 4000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-02", Currency: "USD", PaidBy: 1, AddedBy: 1,
	}, []store.ExpenseParticipant{{UserID: 1, Amount: 2000}, {UserID: 2, Amount: -2000}})
	if err != nil {
		t.Fatal(err)
	}
	g, err := h.st.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: 1, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{3, 4} {
		if err := h.st.AddGroupMember(ctx, g.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	hotel, err := h.st.CreateExpense(ctx, &store.Expense{
		Name: "Hotel", Category: "general", Amount: 6000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-03", Currency: "USD", PaidBy: 1, AddedBy: 1,
		GroupID: sql.NullInt64{Int64: g.ID, Valid: true},
	}, []store.ExpenseParticipant{{UserID: 1, Amount: 3000}, {UserID: 4, Amount: -3000}})
	if err != nil {
		t.Fatal(err)
	}

	check := func(who string, e *store.Expense, want int) {
		t.Helper()
		resp := h.get("/expenses/" + e.ID)
		b := body(t, resp)
		if resp.StatusCode != want {
			t.Errorf("%s opening %q = %d, want %d", who, e.Name, resp.StatusCode, want)
		}
		if want == http.StatusNotFound && strings.Contains(b, e.Name) {
			t.Errorf("%s's 404 for %q still shows it", who, e.Name)
		}
	}
	check("a stranger", direct, http.StatusNotFound)
	check("a stranger", hotel, http.StatusNotFound)
	h.login("dave@example.com", "password123") // in the group, not in either expense
	check("a group member", hotel, http.StatusOK)
	check("a group member", direct, http.StatusNotFound)
	h.login("bob@example.com", "password123") // a participant
	check("a participant", direct, http.StatusOK)
}
