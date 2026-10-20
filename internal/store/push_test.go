package store

import (
	"context"
	"sort"
	"testing"
)

// TestListPushSubscriptionsForSeveralUsers: a notification to several people
// reaches every device each of them registered, and nobody else's.
func TestListPushSubscriptionsForSeveralUsers(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a, b, c := covUser(t, st, "A"), covUser(t, st, "B"), covUser(t, st, "C")
	for _, s := range []struct {
		user     int64
		endpoint string
	}{{a.ID, "a-phone"}, {a.ID, "a-laptop"}, {b.ID, "b-phone"}, {c.ID, "c-phone"}} {
		if err := st.SavePushSubscription(ctx, s.user, s.endpoint, `{}`); err != nil {
			t.Fatal(err)
		}
	}

	subs, err := st.ListPushSubscriptions(ctx, []int64{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, s.Endpoint)
	}
	sort.Strings(got)
	if want := []string{"a-laptop", "a-phone", "b-phone"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("subscriptions for A and B = %v, want %v", got, want)
	}
}
