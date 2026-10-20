package httpapp

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// outsiderHarness is tripHarness (Alice 1 and Bob 2 in group 1, Bob paid the
// Hotel) plus Carol (3), a registered account in no group. The session is left as
// Carol and redirects are not followed, so a refusal shows its real status.
func outsiderHarness(t *testing.T) *harness {
	t.Helper()
	h := tripHarness(t)
	h.register("Carol", "carol@example.com", "password123") // 3, session now Carol
	covNoRedirect(h)
	return h
}

// countOf runs a COUNT(*) query that takes no arguments.
func countOf(t *testing.T, h *harness, query string) int {
	t.Helper()
	var n int
	if err := h.st.DB.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// groupFlags reads the two flags the group toggles flip.
func groupFlags(t *testing.T, h *harness) (archived, simplify bool) {
	t.Helper()
	var arch sql.NullString
	if err := h.st.DB.QueryRow(`SELECT archived_at, simplify_debts FROM groups WHERE id = 1`).Scan(&arch, &simplify); err != nil {
		t.Fatalf("read group flags: %v", err)
	}
	return arch.Valid, simplify
}

// TestGroupActionsRejectOutsider pins the membership check on every group-level
// action: a signed-in user outside the group gets 403 and nothing is written --
// the flags keep their values, nobody is invited (no pending account is even
// created) and no history is collapsed. Group ids are sequential, so without the
// check any account could walk them.
func TestGroupActionsRejectOutsider(t *testing.T) {
	h := outsiderHarness(t)
	archived0, simplify0 := groupFlags(t, h)
	members0 := countOf(t, h, `SELECT COUNT(*) FROM group_users WHERE group_id = 1`)
	users0 := countOf(t, h, `SELECT COUNT(*) FROM users`)
	live0 := countOf(t, h, `SELECT COUNT(*) FROM expenses WHERE deleted_at IS NULL`)

	cases := []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodPost, "/groups/1/archive", url.Values{}},
		{http.MethodPost, "/groups/1/simplify", url.Values{}},
		{http.MethodPost, "/groups/1/invite", url.Values{"email": {"dave@example.com"}}},
		{http.MethodPost, "/groups/1/invite", url.Values{"friend_id": {"2"}}},
		{http.MethodGet, "/groups/1/collapse?before=2099-01-01", nil},
		{http.MethodPost, "/groups/1/collapse", url.Values{"before": {"2099-01-01"}}},
	}
	for _, tc := range cases {
		var resp *http.Response
		if tc.method == http.MethodGet {
			resp = h.get(tc.path)
		} else {
			resp = h.post(tc.path, tc.form)
		}
		b := body(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s status %d, want 403", tc.method, tc.path, resp.StatusCode)
		}
		if strings.Contains(b, "Trip") {
			t.Errorf("%s %s reveals the group name", tc.method, tc.path)
		}
	}

	if archived, simplify := groupFlags(t, h); archived != archived0 || simplify != simplify0 {
		t.Errorf("group flags changed: archived %v->%v, simplify %v->%v", archived0, archived, simplify0, simplify)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM group_users WHERE group_id = 1`); n != members0 {
		t.Errorf("group members %d -> %d, want unchanged", members0, n)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM users`); n != users0 {
		t.Errorf("users %d -> %d: a refused invite still created an account", users0, n)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM expenses WHERE deleted_at IS NULL`); n != live0 {
		t.Errorf("live expenses %d -> %d: a refused collapse still rewrote history", live0, n)
	}
}

// TestGroupMembersHiddenFromOutsider pins the member-list gate in
// candidatesForContext: whichever route names a group -- the new-expense form,
// the edit form's target, or either form coming back with an error -- someone
// outside it is offered their own people, never that group's member names or
// emails. Carol is the payer of her own direct expense, so the edit routes
// render for her and only the gate stands between her and group 1's members.
func TestGroupMembersHiddenFromOutsider(t *testing.T) {
	h := outsiderHarness(t)
	h.post("/friends/add", url.Values{"email": {"dave@example.com"}}) // 4
	h.post("/expenses", url.Values{
		"name": {"Lunch"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "method": {"EQUAL"}, "paid_by": {"3"},
		"include_3": {"1"}, "include_4": {"1"},
	})
	var lunch string
	if err := h.st.DB.QueryRow(`SELECT id FROM expenses WHERE name = 'Lunch'`).Scan(&lunch); err != nil {
		t.Fatalf("find Lunch: %v", err)
	}
	badEdit := url.Values{
		"name": {"Lunch"}, "amount": {"not-a-number"}, "currency": {"USD"},
		"method": {"EQUAL"}, "paid_by": {"3"}, "include_3": {"1"}, "target_group": {"1"},
	}
	badAdd := url.Values{
		"name": {"Snacks"}, "amount": {"not-a-number"}, "currency": {"USD"},
		"method": {"EQUAL"}, "paid_by": {"3"}, "include_3": {"1"}, "group_id": {"1"},
	}

	check := func(label string, resp *http.Response, want int) {
		t.Helper()
		b := body(t, resp)
		if resp.StatusCode != want {
			t.Errorf("%s status %d, want %d", label, resp.StatusCode, want)
		}
		for _, leak := range []string{"Alice", "bob@example.com"} {
			if strings.Contains(b, leak) {
				t.Errorf("%s lists group member %q to an outsider", label, leak)
			}
		}
	}
	check("GET /expenses/new?group=1", h.get("/expenses/new?group=1"), http.StatusOK)
	check("GET move?target=1", h.get("/expenses/"+lunch+"/move?target=1"), http.StatusOK)
	check("POST move error re-render", h.post("/expenses/"+lunch+"/move", badEdit), http.StatusBadRequest)
	check("POST /expenses error re-render", h.post("/expenses", badAdd), http.StatusBadRequest)

	// Control: a member still gets the group's people on the same form. login
	// expects to follow the post-login redirect, so restore that first.
	h.client.CheckRedirect = nil
	h.login("alice@example.com", "password123")
	if b := body(t, h.get("/expenses/new?group=1")); !strings.Contains(b, "bob@example.com") {
		t.Error("a group member no longer sees the group's members on the new-expense form")
	}
}
