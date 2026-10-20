package store

import (
	"context"
	"testing"
)

// TestAreRelated: two people are related through a friendship, a shared group,
// or a shared expense -- a deleted one included, since its history still shows
// -- and through nothing else.
func TestAreRelated(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a, friend, mate, payee, stranger := covUser(t, st, "a"), covUser(t, st, "friend"), covUser(t, st, "mate"),
		covUser(t, st, "payee"), covUser(t, st, "stranger")
	if err := st.AddFriend(ctx, a.ID, friend.ID); err != nil {
		t.Fatal(err)
	}
	g, err := st.CreateGroup(ctx, &Group{Name: "Flat", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGroupMember(ctx, g.ID, mate.ID); err != nil {
		t.Fatal(err)
	}
	e := covDirectExpense(t, st, "Old", "2026-01-01", a.ID,
		[]ExpenseParticipant{{UserID: a.ID, Amount: 300}, {UserID: payee.ID, Amount: -300}})
	if err := st.SoftDeleteExpense(ctx, e.ID, a.ID, e.Version); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		b    *User
		want bool
	}{
		{"friend", friend, true},
		{"group mate", mate, true},
		{"shared a deleted expense", payee, true},
		{"stranger", stranger, false},
	} {
		for _, pair := range [][2]int64{{a.ID, tc.b.ID}, {tc.b.ID, a.ID}} {
			got, err := st.AreRelated(ctx, pair[0], pair[1])
			if err != nil || got != tc.want {
				t.Errorf("%s: AreRelated(%d, %d) = %v, %v; want %v", tc.name, pair[0], pair[1], got, err, tc.want)
			}
		}
	}
}
