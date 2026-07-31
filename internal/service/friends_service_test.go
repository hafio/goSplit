package service

import (
	"context"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// TestAddFriendToGroup covers the "add an existing friend by name" path: a member
// can add a friend, a non-friend is rejected (defense-in-depth for hand-crafted
// POSTs), and a non-member can't add anyone.
func TestAddFriendToGroup(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	alice, _ := svc.Register(ctx, "Alice", "alice@example.com", "password12")
	bob, _ := svc.Register(ctx, "Bob", "bob@example.com", "password12")
	carol, _ := svc.Register(ctx, "Carol", "carol@example.com", "password12")

	// Alice and Bob are friends; Alice owns and belongs to a group.
	if err := svc.Store.AddFriend(ctx, alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: alice.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.AddGroupMember(ctx, g.ID, alice.ID); err != nil {
		t.Fatal(err)
	}

	// (a) A member adds a friend -> the friend joins the group.
	if err := svc.AddFriendToGroup(ctx, alice, g.ID, bob.ID); err != nil {
		t.Fatalf("member adding a friend should succeed: %v", err)
	}
	if ok, _ := svc.Store.IsGroupMember(ctx, g.ID, bob.ID); !ok {
		t.Fatal("bob should be a group member")
	}

	// (b) Adding someone who isn't a friend is rejected and not added.
	if err := svc.AddFriendToGroup(ctx, alice, g.ID, carol.ID); err == nil {
		t.Fatal("adding a non-friend should error")
	}
	if ok, _ := svc.Store.IsGroupMember(ctx, g.ID, carol.ID); ok {
		t.Fatal("carol must not have been added")
	}

	// (c) A non-member actor can't add others, even their own friend.
	dave, _ := svc.Register(ctx, "Dave", "dave@example.com", "password12")
	if err := svc.Store.AddFriend(ctx, dave.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddFriendToGroup(ctx, dave, g.ID, bob.ID); err == nil {
		t.Fatal("a non-member must not add others")
	}
}
