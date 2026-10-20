package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestAdminViewsAnyGroupReadOnly: an admin outside a group can open its page --
// balances and feed included -- but gets none of the member-only controls, no
// join link (it would let them in), and a private response; every group-level
// action is still refused, and nothing changes.
func TestAdminViewsAnyGroupReadOnly(t *testing.T) {
	h := adminOutsiderHarness(t)
	g, err := h.st.GetGroup(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	resp := h.get("/groups/1")
	page := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET /groups/1 = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Error("read-only group page is not marked private")
	}
	for _, want := range []string{"Hotel", "Admin view", `id="group-feed"`, "50.00 USD"} {
		if !strings.Contains(page, want) {
			t.Errorf("read-only group page missing %q", want)
		}
	}
	for _, control := range []string{
		"/groups/1/simplify", "/groups/1/archive", "/groups/1/collapse", "/groups/1/invite",
		"/expenses/new?group=1", "/groups/1/settle", "/groups/1/tidy", "/g/" + g.PublicID, g.PublicID,
	} {
		if strings.Contains(page, control) {
			t.Errorf("read-only group page still offers %q", control)
		}
	}

	archived0, simplify0 := groupFlags(t, h)
	members0 := countOf(t, h, `SELECT COUNT(*) FROM group_users WHERE group_id = 1`)
	covNoRedirect(h)
	for _, tc := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodPost, "/groups/1/archive", url.Values{}},
		{http.MethodPost, "/groups/1/simplify", url.Values{}},
		{http.MethodPost, "/groups/1/invite", url.Values{"email": {"dave@example.com"}}},
		{http.MethodGet, "/groups/1/settle", nil},
		{http.MethodPost, "/groups/1/settle", url.Values{}},
		{http.MethodGet, "/groups/1/collapse", nil},
		{http.MethodPost, "/groups/1/collapse", url.Values{"before": {"2099-01-01"}}},
	} {
		var resp *http.Response
		if tc.method == http.MethodGet {
			resp = h.get(tc.path)
		} else {
			resp = h.post(tc.path, tc.form)
		}
		_ = body(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("non-member admin %s %s = %d, want 403", tc.method, tc.path, resp.StatusCode)
		}
	}
	if archived, simplify := groupFlags(t, h); archived != archived0 || simplify != simplify0 {
		t.Error("a refused admin action changed the group's flags")
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM group_users WHERE group_id = 1`); n != members0 {
		t.Errorf("group members %d -> %d after a refused admin invite", members0, n)
	}
	if n := countOf(t, h, `SELECT COUNT(*) FROM expenses WHERE split_type = 'SETTLEMENT'`); n != 0 {
		t.Errorf("%d settlements after a refused admin settle-up", n)
	}
}

// TestMemberAdminKeepsGroupControls: being an admin does not make one's own
// group read-only.
func TestMemberAdminKeepsGroupControls(t *testing.T) {
	h := newHarness(t)
	h.register("Root", "admin@example.com", "password123")
	h.post("/groups/create", url.Values{"name": {"Home"}, "currency": {"USD"}})
	page := body(t, h.get("/groups/1"))
	if strings.Contains(page, "Admin view") {
		t.Error("an admin's own group is shown read-only")
	}
	for _, want := range []string{"/groups/1/simplify", "/groups/1/invite", "/expenses/new?group=1"} {
		if !strings.Contains(page, want) {
			t.Errorf("member admin lost the control %q", want)
		}
	}
}

// TestAdminGroupList: /admin/groups lists every group with its member count and
// a link into the admin feed for it, and never shows a group's join token.
func TestAdminGroupList(t *testing.T) {
	h := adminOutsiderHarness(t)
	g, err := h.st.GetGroup(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	resp := h.get("/admin/groups")
	page := body(t, resp)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-GoSplit-Private") != "1" {
		t.Fatalf("admin groups = %d (private %q), want 200 and private", resp.StatusCode, resp.Header.Get("X-GoSplit-Private"))
	}
	for _, want := range []string{`href="/groups/1">Trip</a>`, "2 members", `href="/admin/activity?group=1"`} {
		if !strings.Contains(page, want) {
			t.Errorf("admin groups missing %q", want)
		}
	}
	if strings.Contains(page, g.PublicID) {
		t.Error("admin groups shows a group's join token")
	}
	// The admin console links to both admin lists.
	admin := body(t, h.get("/admin"))
	for _, link := range []string{`href="/admin/activity"`, `href="/admin/groups"`, `href="/admin/activity?user=1"`} {
		if !strings.Contains(admin, link) {
			t.Errorf("admin console missing %q", link)
		}
	}
}
