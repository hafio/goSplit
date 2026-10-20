package httpapp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

const limitedBody = "Limited accounts cannot create groups or import from Splitwise. Ask an admin to make your account a full user."

// limitedHarness has the admin create Lim (2) through the console with no role
// -- the default, LIMITED -- and a password, then signs in as Lim. Redirects are
// not followed so refusals show their real status.
func limitedHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123") // 1, ADMIN via AdminEmails
	_ = body(t, h.post("/admin/users/create", url.Values{
		"name": {"Lim"}, "email": {"lim@example.com"}, "password": {"password123"},
		"currency": {"USD"}, "language": {"en"},
	}))
	if u := userByEmail(t, h, "lim@example.com"); u.Role != store.RoleLimited {
		t.Fatalf("admin-created role = %q, want LIMITED", u.Role)
	}
	h.login("lim@example.com", "password123")
	covNoRedirect(h)
	return h
}

// TestLimitedCannotCreateGroup pins the group-create gate end to end: a LIMITED
// account is answered 403 with the explanation page and no group row appears.
func TestLimitedCannotCreateGroup(t *testing.T) {
	h := limitedHarness(t)
	resp := h.post("/groups/create", url.Values{"name": {"Nope"}, "currency": {"USD"}})
	b := body(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("LIMITED group create status %d, want 403", resp.StatusCode)
	}
	for _, want := range []string{"Not available for limited accounts", limitedBody} {
		if !strings.Contains(b, want) {
			t.Errorf("refusal page missing %q", want)
		}
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM groups`); n != 0 {
		t.Errorf("%d groups after a refused create, want 0", n)
	}
}

// TestLimitedPagesHideGroupCreationAndImport covers the UI side: no create form
// on /groups, no Import entry in the menu, and the import page and its POST both
// answer with the explanation. Promoting the account brings both back on the
// next request, because the session reloads the user every time.
func TestLimitedPagesHideGroupCreationAndImport(t *testing.T) {
	h := limitedHarness(t)
	groups := body(t, h.get("/groups"))
	if strings.Contains(groups, `action="/groups/create"`) {
		t.Error("LIMITED /groups page still offers the create form")
	}
	if strings.Contains(groups, `href="/import"`) {
		t.Error("LIMITED menu still links to the import page")
	}
	for _, tc := range []struct {
		method, path string
	}{{http.MethodGet, "/import"}, {http.MethodPost, "/import/splitwise"}} {
		var resp *http.Response
		if tc.method == http.MethodGet {
			resp = h.get(tc.path)
		} else {
			resp = h.post(tc.path, url.Values{"api_key": {"some-key"}})
		}
		b := body(t, resp)
		if resp.StatusCode != http.StatusForbidden || !strings.Contains(b, limitedBody) {
			t.Errorf("%s %s = %d without the explanation, want 403 with it", tc.method, tc.path, resp.StatusCode)
		}
	}

	lim := userByEmail(t, h, "lim@example.com")
	if err := h.st.SetRole(context.Background(), lim.ID, store.RoleUser); err != nil {
		t.Fatal(err)
	}
	groups = body(t, h.get("/groups"))
	if !strings.Contains(groups, `action="/groups/create"`) || !strings.Contains(groups, `href="/import"`) {
		t.Error("promoted account still has group creation or import hidden")
	}
}

// TestAdminConsoleLimitedRole covers the console side of the new role: the Add
// user form preselects LIMITED, an edit to LIMITED persists (checked in the
// store -- the flash shows even when a role is ignored), the badge appears, and
// the edit form keeps LIMITED selected so saving another field cannot silently
// promote the account to USER.
func TestAdminConsoleLimitedRole(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")

	page := body(t, h.get("/admin"))
	if !strings.Contains(page, `<option value="LIMITED" selected>LIMITED</option>`) {
		t.Error("Add user form does not preselect LIMITED")
	}
	if !strings.Contains(page, "Accounts you add start as LIMITED.") {
		t.Error("Add user form missing the role hint")
	}

	_ = body(t, h.post("/admin/users/create", url.Values{
		"name": {"Full"}, "email": {"full@example.com"}, "role": {"USER"},
		"currency": {"USD"}, "language": {"en"},
	}))
	full := userByEmail(t, h, "full@example.com")
	if full.Role != store.RoleUser {
		t.Fatalf("explicit USER create gave %q", full.Role)
	}
	_ = body(t, h.post(fmt.Sprintf("/admin/users/%d", full.ID), url.Values{
		"name": {"Full"}, "email": {"full@example.com"}, "role": {"LIMITED"},
		"currency": {"USD"}, "language": {"en"},
	}))
	if got := userByEmail(t, h, "full@example.com"); got.Role != store.RoleLimited {
		t.Fatalf("admin edit to LIMITED stored %q", got.Role)
	}

	page = body(t, h.get("/admin"))
	if !strings.Contains(page, `<span class="tag">LIMITED</span>`) {
		t.Error("LIMITED account has no badge in the user list")
	}
	// Two preselected LIMITED options: the Add user form's and Full's edit form.
	if n := strings.Count(page, `<option value="LIMITED" selected>LIMITED</option>`); n != 2 ||
		strings.Contains(page, `<option value="USER" selected>`) {
		t.Errorf("edit form does not keep LIMITED selected for the LIMITED account (%d preselected)", n)
	}

	// An unknown role is ignored, not stored.
	_ = body(t, h.post(fmt.Sprintf("/admin/users/%d", full.ID), url.Values{
		"name": {"Full"}, "email": {"full@example.com"}, "role": {"SUPER"},
		"currency": {"USD"}, "language": {"en"},
	}))
	if got := userByEmail(t, h, "full@example.com"); got.Role != store.RoleLimited {
		t.Errorf("unknown role overwrote LIMITED with %q", got.Role)
	}
}
