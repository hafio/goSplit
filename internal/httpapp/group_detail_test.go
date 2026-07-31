package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestGroupDetailPolish checks the refreshed group page: identity header with a
// member avatar stack, the overflow menu holding archive/simplify, and a settle
// button on a settlement the current user owes.
func TestGroupDetailPolish(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}}) // Bob must be a member to be a participant

	// Bob(2) pays for everyone → Alice(1) owes Bob, so Alice sees a Settle button.
	h.post("/expenses", url.Values{
		"name": {"Hotel"}, "amount": {"100.00"}, "currency": {"USD"},
		"date": {"2026-03-10"}, "method": {"EQUAL"}, "paid_by": {"2"},
		"group_id": {"1"}, "include_1": {"1"}, "include_2": {"1"},
	})

	b := body(t, h.get("/groups/1"))
	for _, want := range []string{`class="ava-stack"`, "icons.svg?v=", "#dots", "#archive", "month-h"} {
		if !strings.Contains(b, want) {
			t.Errorf("group page missing %q", want)
		}
	}
	// Alice owes Bob → settle link scoped to this group, so the payment clears the
	// group's own balance (see settle_test.go).
	if !strings.Contains(b, `/groups/1/settle/2`) {
		t.Errorf("expected a settle button linking to /groups/1/settle/2")
	}

	// Overflow archive form still works (posts and redirects).
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if resp := h.post("/groups/1/archive", url.Values{}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("archive status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}
}

// TestGroupAddFriendByPicker checks the "add a member by picking a friend" flow:
// the group page offers a friend dropdown, posting friend_id adds that friend, and
// once added the friend is no longer offered.
func TestGroupAddFriendByPicker(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}}) // Bob = user 2, now Alice's friend
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})

	// The group page offers a friend picker listing Bob (a friend, not yet a member).
	page := body(t, h.get("/groups/1"))
	if !strings.Contains(page, `name="friend_id"`) {
		t.Fatal("group page missing the friend picker")
	}
	if !strings.Contains(page, "bob@example.com") {
		t.Fatalf("friend picker missing bob: %s", page)
	}

	// Adding Bob via the picker makes him a group member.
	h.post("/groups/1/invite", url.Values{"friend_id": {"2"}})
	if ok, _ := h.st.IsGroupMember(context.Background(), 1, 2); !ok {
		t.Fatal("bob should be a group member after the picker add")
	}

	// Bob was Alice's only friend, so with him in the group the picker is gone.
	page = body(t, h.get("/groups/1"))
	if strings.Contains(page, `name="friend_id"`) {
		t.Error("picker should be absent once the only addable friend has joined")
	}
}
