package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// covLimitedUser creates a LIMITED account, the kind an invite or an admin's
// "Add user" produces.
func covLimitedUser(t *testing.T, svc *Service, name, email string) *store.User {
	t.Helper()
	u, err := svc.Store.CreateUser(context.Background(), &store.User{Name: name, Email: email, Role: store.RoleLimited})
	if err != nil {
		t.Fatalf("create limited user %s: %v", email, err)
	}
	return u
}

// TestCreateGroupRefusesLimited pins the one gate in front of Store.CreateGroup:
// a LIMITED actor gets ErrLimitedUser and nothing is written, while a full
// account gets the group with itself as creator and first member -- whatever
// CreatedBy the caller passed in.
func TestCreateGroupRefusesLimited(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	lim := covLimitedUser(t, svc, "Lim", "lim@x.com")

	if g, err := svc.CreateGroup(ctx, lim, &store.Group{Name: "Nope"}); err != ErrLimitedUser || g != nil {
		t.Fatalf("CreateGroup(LIMITED) = %v, %v; want nil, ErrLimitedUser", g, err)
	}
	if n := countRows(t, svc, "groups"); n != 0 {
		t.Fatalf("%d group rows after a refused create, want 0", n)
	}
	if n := countRows(t, svc, "group_users"); n != 0 {
		t.Fatalf("%d memberships after a refused create, want 0", n)
	}

	full := covUser(t, svc, "Full", "full@x.com")
	g, err := svc.CreateGroup(ctx, full, &store.Group{Name: "Trip", CreatedBy: lim.ID})
	if err != nil {
		t.Fatalf("CreateGroup(USER): %v", err)
	}
	if g.CreatedBy != full.ID {
		t.Errorf("CreatedBy = %d, want the actor %d", g.CreatedBy, full.ID)
	}
	if ok, _ := svc.Store.IsGroupMember(ctx, g.ID, full.ID); !ok {
		t.Error("the creator should be the group's first member")
	}
}

// TestImportRefusesLimitedBeforeAnyRequest pins the import gate: an import
// creates groups, so a LIMITED actor is refused with ErrLimitedUser before the
// key is even checked -- Splitwise never sees a request and no friend, account or
// group is written.
func TestImportRefusesLimitedBeforeAnyRequest(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	lim := covLimitedUser(t, svc, "Lim", "lim@x.com")

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/get_friends":
			_, _ = w.Write([]byte(`{"friends":[{"email":"pal@x.com","first_name":"Pal"}]}`))
		case "/get_groups":
			_, _ = w.Write([]byte(`{"groups":[{"id":9,"name":"Trip","members":[]}]}`))
		}
	}))
	defer srv.Close()
	orig := splitwiseBase
	splitwiseBase = srv.URL
	defer func() { splitwiseBase = orig }()

	users0 := countRows(t, svc, "users")
	for _, key := range []string{"some-key", ""} {
		if res, err := svc.ImportFromSplitwise(ctx, lim, key); err != ErrLimitedUser || res != nil {
			t.Errorf("import(key %q) = %v, %v; want nil, ErrLimitedUser", key, res, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Splitwise got %d request(s) from a refused import, want 0", n)
	}
	if n := countRows(t, svc, "groups"); n != 0 {
		t.Errorf("%d groups after a refused import, want 0", n)
	}
	if n := countRows(t, svc, "friendships"); n != 0 {
		t.Errorf("%d friend rows after a refused import, want 0", n)
	}
	if n := countRows(t, svc, "users"); n != users0 {
		t.Errorf("users %d -> %d after a refused import, want unchanged", users0, n)
	}
}
