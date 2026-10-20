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

// adminOutsiderHarness is tripHarness (Alice 1 and Bob 2 in group 1, Bob paid
// the 100.00 Hotel split with Alice) with an admin registered last (3), who is
// in neither the group nor the expense. The session is the admin's.
func adminOutsiderHarness(t *testing.T) *harness {
	t.Helper()
	h := tripHarness(t)
	h.register("Root", "admin@example.com", "password123") // 3, ADMIN via AdminEmails
	return h
}

// expenseByName returns the one expense row with that name.
func expenseByName(t *testing.T, h *harness, name string) *store.Expense {
	t.Helper()
	var id string
	if err := h.st.DB.QueryRow(h.st.Rebind(`SELECT id FROM expenses WHERE name = ?`), name).Scan(&id); err != nil {
		t.Fatalf("find expense %q: %v", name, err)
	}
	e, err := h.st.GetExpense(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestAdminActivityRequiresAdmin: the all-users feed is behind RequireAdmin --
// signed out is sent to sign in, a plain user is refused.
func TestAdminActivityRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	covNoRedirect(h)
	resp := h.get("/admin/activity")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/login") {
		t.Fatalf("signed out: %d to %q, want 303 to /login", resp.StatusCode, resp.Header.Get("Location"))
	}
	h.client.CheckRedirect = nil
	h.register("Alice", "alice@example.com", "password123")
	covNoRedirect(h)
	for _, path := range []string{"/admin/activity", "/admin/groups"} {
		resp = h.get(path)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("plain user GET %s = %d, want 403", path, resp.StatusCode)
		}
	}
}

// TestAdminActivityListsEveryoneNeutrally: the admin sees every user's
// transactions, a deleted one dimmed and tagged, each row with its plain total
// (no "you lent" for someone who is not part of it), rows navigating the page,
// and the response marked private for the service worker.
func TestAdminActivityListsEveryoneNeutrally(t *testing.T) {
	h := tripHarness(t)
	h.post("/expenses", url.Values{
		"name": {"Coffee"}, "amount": {"8.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
		"method": {"EQUAL"}, "paid_by": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	coffee := expenseByName(t, h, "Coffee")
	h.post("/expenses/"+coffee.ID+"/delete", url.Values{"version": {fmt.Sprint(coffee.Version)}})
	h.register("Root", "admin@example.com", "password123")

	resp := h.get("/admin/activity")
	page := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin activity status %d", resp.StatusCode)
	}
	if resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Error("admin activity is not marked private")
	}
	for _, want := range []string{"Hotel", "Coffee", `class="l-row dim"`, `class="tag danger"`, `hx-target="body"`, `id="admin-activity-feed"`} {
		if !strings.Contains(page, want) {
			t.Errorf("admin activity missing %q", want)
		}
	}
	for _, unwanted := range []string{"you lent", "you borrowed", "<span class=\"amt muted\">\u2014</span>"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("admin activity shows viewer-relative %q", unwanted)
		}
	}
	// Neutral rows show each transaction's total instead.
	if !strings.Contains(page, `<span class="amt muted">100.00</span>`) {
		t.Error("admin activity rows do not show the Hotel's total")
	}
}

// TestAdminActivityFilters: ?user= keeps only that user's own transactions,
// ?group= that group's, they combine with the search box, the chips name the
// person and drop only their own parameter, Clear-all drops everything, and a
// malformed or unknown id is ignored rather than emptying the feed.
func TestAdminActivityFilters(t *testing.T) {
	h := tripHarness(t)
	h.post("/expenses", url.Values{
		"name": {"Coffee"}, "amount": {"8.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
		"method": {"EQUAL"}, "paid_by": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	h.register("Root", "admin@example.com", "password123")

	has := func(path string, want, unwanted []string) string {
		t.Helper()
		page := body(t, h.get(path))
		for _, w := range want {
			if !strings.Contains(page, w) {
				t.Errorf("GET %s missing %q", path, w)
			}
		}
		for _, u := range unwanted {
			if strings.Contains(page, u) {
				t.Errorf("GET %s has %q", path, u)
			}
		}
		return page
	}
	has("/admin/activity?group=1", []string{"Hotel"}, []string{"Coffee"})
	page := has("/admin/activity?user=1&q=Cof", []string{"Coffee"}, []string{"Hotel"})
	for _, chip := range []string{
		`href="/admin/activity?q=Cof">Alice`,       // the user chip keeps the search
		`href="/admin/activity?user=1">`,           // the search chip keeps the user
		`<a class="chip clear-chip" href="/admin/activity">`,
		`<option value="1" selected>Alice (alice@example.com)</option>`,
	} {
		if !strings.Contains(page, chip) {
			t.Errorf("filtered page missing %q", chip)
		}
	}
	for _, junk := range []string{"/admin/activity?user=abc", "/admin/activity?user=999&group=999"} {
		has(junk, []string{"Hotel", "Coffee"}, nil)
	}
}

// TestAdminEditsAnyonesExpenseUntouched: an admin outside the group opens the
// edit form of a group expense -- the current group is selected and the stored
// participants are there although the admin has no friends -- and an untouched
// save goes through without an acknowledgment and leaves the split exactly as
// it was, stamped with the admin.
func TestAdminEditsAnyonesExpenseUntouched(t *testing.T) {
	h := adminOutsiderHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	resp := h.get("/expenses/" + hotel.ID + "/move")
	form := body(t, resp)
	if resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Error("admin edit form is not marked private")
	}
	for _, want := range []string{`<option value="1" selected>Trip</option>`, `name="include_1"`, `name="include_2"`} {
		if !strings.Contains(form, want) {
			t.Errorf("admin edit form missing %q", want)
		}
	}
	covNoRedirect(h)
	resp = h.post("/expenses/"+hotel.ID+"/move", url.Values{
		"name": {"Hotel"}, "category": {"general"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"}, "target_group": {"1"},
		"include_1": {"1"}, "include_2": {"1"}, "version": {fmt.Sprint(hotel.Version)},
	})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin untouched save status %d, want 303", resp.StatusCode)
	}
	after := expenseByName(t, h, "Hotel")
	if !after.GroupID.Valid || after.GroupID.Int64 != 1 || after.UpdatedBy.Int64 != 3 {
		t.Errorf("after admin save: group %+v, updated_by %+v", after.GroupID, after.UpdatedBy)
	}
	parts, _ := h.st.GetParticipants(context.Background(), hotel.ID)
	got := map[int64]int64{}
	for _, p := range parts {
		got[p.UserID] = p.Amount
	}
	if len(got) != 2 || got[1] != -5000 || got[2] != 5000 {
		t.Errorf("split changed by an untouched admin save: %v", got)
	}

	// The detail page says the admin is acting on someone else's transaction.
	h.client.CheckRedirect = nil
	resp = h.get("/expenses/" + hotel.ID)
	detail := body(t, resp)
	if !strings.Contains(detail, "You are acting as an admin") || resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Error("detail page missing the admin notice or the private marker")
	}
	// The admin still sees the group's members on the add form (members-or-admin),
	// and that page is private too.
	resp = h.get("/expenses/new?group=1")
	if b := body(t, resp); !strings.Contains(b, "bob@example.com") || resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Error("an admin should see the group's members on the expense form, on a private page")
	}
	// A member's own form is an ordinary page.
	h.login("alice@example.com", "password123")
	if resp = h.get("/expenses/" + hotel.ID + "/move"); resp.Header.Get("X-GoSplit-Private") != "" {
		t.Error("a member's edit form is marked private")
	}
	_ = body(t, resp)
}

// TestAdminEditsDirectExpenseUntouched covers the direct-expense branch of the
// edit form for an admin who is not part of it: the people offered are the
// stored participants (not the admin, who has no friends here), and an untouched
// save keeps the split exactly as it was.
func TestAdminEditsDirectExpenseUntouched(t *testing.T) {
	h := tripHarness(t)
	h.post("/expenses", url.Values{
		"name": {"Coffee"}, "amount": {"8.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
		"method": {"EQUAL"}, "paid_by": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})
	coffee := expenseByName(t, h, "Coffee")
	h.register("Root", "admin@example.com", "password123") // 3
	form := body(t, h.get("/expenses/" + coffee.ID + "/move"))
	if !strings.Contains(form, `name="include_1"`) || !strings.Contains(form, `name="include_2"`) {
		t.Error("admin edit form of a direct expense misses a stored participant")
	}
	if strings.Contains(form, `name="include_3"`) {
		t.Error("admin edit form offers the admin as a participant")
	}
	covNoRedirect(h)
	resp := h.post("/expenses/"+coffee.ID+"/move", url.Values{
		"name": {"Coffee"}, "category": {"general"}, "amount": {"8.00"}, "currency": {"USD"},
		"date": {"2026-03-11"}, "method": {"EQUAL"}, "paid_by": {"1"}, "target_group": {"none"},
		"include_1": {"1"}, "include_2": {"1"}, "version": {fmt.Sprint(coffee.Version)},
	})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin untouched direct save status %d, want 303", resp.StatusCode)
	}
	parts, _ := h.st.GetParticipants(context.Background(), coffee.ID)
	got := map[int64]int64{}
	for _, p := range parts {
		got[p.UserID] = p.Amount
	}
	if len(got) != 2 || got[1] != 400 || got[2] != -400 {
		t.Errorf("split changed by an untouched admin save: %v", got)
	}
}

// TestAdminActivityShowAllKeepsFilters: on a feed long enough to be truncated,
// the "Show all" link carries the user and group filters along.
func TestAdminActivityShowAllKeepsFilters(t *testing.T) {
	h := tripHarness(t)
	for i := 0; i < feedLimit+1; i++ {
		h.post("/expenses", url.Values{
			"name": {fmt.Sprintf("Bulk %02d", i)}, "amount": {"1.00"}, "currency": {"USD"}, "date": {"2026-03-11"},
			"method": {"EQUAL"}, "paid_by": {"1"}, "group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
		})
	}
	h.register("Root", "admin@example.com", "password123")
	page := body(t, h.get("/admin/activity?user=1&group=1"))
	if !strings.Contains(page, `href="/admin/activity?all=1&amp;group=1&amp;user=1"`) {
		t.Error("show-all link dropped the user or group filter")
	}
	if all := body(t, h.get("/admin/activity?all=1&group=1&user=1")); strings.Contains(all, "/admin/activity?all=1") {
		t.Error("the full feed still offers show-all")
	}
}

// TestBuildRowsViewerZeroIsNeutral pins the viewer-0 short-circuit the admin
// feed relies on: no per-viewer net is attached.
func TestBuildRowsViewerZeroIsNeutral(t *testing.T) {
	h := tripHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	nc := h.app.newNameCache()
	if rows := h.app.buildRows(context.Background(), nc, 1, []*store.Expense{hotel}); !rows[0].HasNet {
		t.Fatal("control: a participant should get a net")
	}
	if rows := h.app.buildRows(context.Background(), nc, 0, []*store.Expense{hotel}); rows[0].HasNet || rows[0].MyNet != 0 {
		t.Errorf("viewer 0 got a per-viewer net: %+v", rows[0])
	}
}

// TestAdminDeleteReturnsToAdminFeed: an admin deleting someone else's expense
// lands back on the admin feed, and the row records who deleted it.
func TestAdminDeleteReturnsToAdminFeed(t *testing.T) {
	h := adminOutsiderHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	covNoRedirect(h)
	resp := h.post("/expenses/"+hotel.ID+"/delete", url.Values{"version": {fmt.Sprint(hotel.Version)}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/admin/activity" {
		t.Fatalf("admin delete: %d to %q, want 303 to /admin/activity", resp.StatusCode, resp.Header.Get("Location"))
	}
	gone := expenseByName(t, h, "Hotel")
	if !gone.DeletedAt.Valid || gone.DeletedBy.Int64 != 3 {
		t.Errorf("delete not recorded against the admin: %+v / %+v", gone.DeletedAt, gone.DeletedBy)
	}
	h.client.CheckRedirect = nil
	if d := body(t, h.get("/expenses/" + hotel.ID)); !strings.Contains(d, "Deleted by Root") {
		t.Error("deleted expense page does not say who deleted it")
	}
	// Deleted rows have no edit form any more: the GET redirects to the detail page.
	covNoRedirect(h)
	resp = h.get("/expenses/" + hotel.ID + "/move")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/expenses/"+hotel.ID {
		t.Errorf("edit form of a deleted row: %d to %q, want 303 to the detail page", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// TestEditFormRefusesNonEditor: someone who may not edit a transaction gets 403
// for its edit form and for a submission, and nothing changes.
func TestEditFormRefusesNonEditor(t *testing.T) {
	h := tripHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	h.register("Carol", "carol@example.com", "password123") // 3, not an admin
	covNoRedirect(h)
	resp := h.get("/expenses/" + hotel.ID + "/move")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-editor GET edit form = %d, want 403", resp.StatusCode)
	}
	resp = h.post("/expenses/"+hotel.ID+"/move", url.Values{
		"name": {"Mine"}, "amount": {"1.00"}, "currency": {"USD"}, "method": {"EQUAL"},
		"paid_by": {"3"}, "include_3": {"1"}, "version": {fmt.Sprint(hotel.Version)},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(b, "part of") {
		t.Errorf("non-editor POST edit = %d, want 403 with the not-editor message", resp.StatusCode)
	}
	if after := expenseByName(t, h, "Hotel"); after.Version != hotel.Version || after.Name != "Hotel" {
		t.Errorf("refused edit still changed the row: %+v", after)
	}
}

// TestEditFormKeepsArchivedCurrentGroup: a member editing an expense of an
// archived group still has that group selected, so an untouched save cannot
// turn into an unacknowledged move out of it. An ARCHIVE row has no form.
func TestEditFormKeepsArchivedCurrentGroup(t *testing.T) {
	h := tripHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	h.post("/groups/1/archive", url.Values{})
	if form := body(t, h.get("/expenses/" + hotel.ID + "/move")); !strings.Contains(form, `<option value="1" selected>Trip</option>`) {
		t.Error("archived current group missing from the edit form's target list")
	}
	covNoRedirect(h)
	resp := h.post("/expenses/"+hotel.ID+"/move", url.Values{
		"name": {"Hotel"}, "category": {"general"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"}, "target_group": {"1"},
		"include_1": {"1"}, "include_2": {"1"}, "version": {fmt.Sprint(hotel.Version)},
	})
	_ = body(t, resp)
	if after := expenseByName(t, h, "Hotel"); resp.StatusCode != http.StatusSeeOther || !after.GroupID.Valid || after.GroupID.Int64 != 1 {
		t.Errorf("untouched save in an archived group: status %d, group %+v; want 303 and group 1 kept", resp.StatusCode, after.GroupID)
	}
	h.client.CheckRedirect = nil

	hist, err := h.st.CreateExpense(context.Background(), &store.Expense{
		Name: "Historical Transactions", Category: "archive", Amount: 100, SplitType: "ARCHIVE",
		ExpenseDate: "2026-01-01", Currency: "USD", PaidBy: 1, AddedBy: 1,
	}, []store.ExpenseParticipant{{UserID: 1, Amount: 100}, {UserID: 2, Amount: -100}})
	if err != nil {
		t.Fatal(err)
	}
	covNoRedirect(h)
	resp = h.get("/expenses/" + hist.ID + "/move")
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("edit form of an ARCHIVE row = %d, want a redirect to its detail page", resp.StatusCode)
	}
}

// TestDeleteStoreFailureIsReported: when the store refuses the soft delete for
// any reason other than the known ones, the user gets an error page, not the
// success redirect, and the row is untouched.
func TestDeleteStoreFailureIsReported(t *testing.T) {
	h := tripHarness(t)
	hotel := expenseByName(t, h, "Hotel")
	if _, err := h.st.DB.Exec(`CREATE TRIGGER no_updates BEFORE UPDATE ON expenses BEGIN SELECT RAISE(ABORT, 'store unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	covNoRedirect(h)
	resp := h.post("/expenses/"+hotel.ID+"/delete", url.Values{"version": {fmt.Sprint(hotel.Version)}})
	b := body(t, resp)
	if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(b, "store unavailable") {
		t.Errorf("failed delete answered %d, want 500 with the cause", resp.StatusCode)
	}
	var deleted sql.NullString
	_ = h.st.DB.QueryRow(h.st.Rebind(`SELECT deleted_at FROM expenses WHERE id = ?`), hotel.ID).Scan(&deleted)
	if deleted.Valid {
		t.Error("row marked deleted although the delete failed")
	}
}

// TestAdminCannotAddToForeignGroup: the override covers existing transactions,
// not adding new ones to a group the admin is not in.
func TestAdminCannotAddToForeignGroup(t *testing.T) {
	h := adminOutsiderHarness(t)
	before := countOf(t, h, `SELECT COUNT(*) FROM expenses`)
	covNoRedirect(h)
	resp := h.post("/expenses", url.Values{
		"name": {"Sneak"}, "amount": {"4.00"}, "currency": {"USD"}, "method": {"EQUAL"},
		"paid_by": {"1"}, "include_1": {"1"}, "include_2": {"1"}, "group_id": {"1"},
	})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("admin add to a foreign group = %d, want 400", resp.StatusCode)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM expenses`); n != before {
		t.Errorf("expenses %d -> %d after a refused add", before, n)
	}
}

// TestServiceWorkerSkipsPrivateResponses pins the offline-cache exclusion: the
// worker never stores the admin area or a response marked private, and its
// cache name moved so old caches holding such pages are purged.
func TestServiceWorkerSkipsPrivateResponses(t *testing.T) {
	h := newHarness(t)
	sw := body(t, h.get("/sw.js"))
	for _, want := range []string{"gosplit-v6", "X-GoSplit-Private", "url.pathname === '/admin'", "url.pathname.startsWith('/admin/')"} {
		if !strings.Contains(sw, want) {
			t.Errorf("sw.js missing %q", want)
		}
	}
	h.register("Alice", "alice@example.com", "password123")
	if resp := h.get("/balances"); resp.Header.Get("X-GoSplit-Private") != "" {
		_ = body(t, resp)
		t.Error("an ordinary page is marked private")
	} else {
		_ = body(t, resp)
	}
}
