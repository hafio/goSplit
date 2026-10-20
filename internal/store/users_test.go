package store

import (
	"context"
	"testing"
)

// TestEmailsStayUnique: an address that differs from an existing one only in
// case or surrounding space is the same address, and is refused both for a new
// account and for an admin renaming another one -- which keeps its own.
func TestEmailsStayUnique(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	if _, err := st.CreateUser(ctx, &User{Name: "Ann", Email: "ann@example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, &User{Name: "Imposter", Email: "  ANN@example.com "}); err == nil {
		t.Error("a second account was created for the same address")
	}

	bob, err := st.CreateUser(ctx, &User{Name: "Bob", Email: "bob@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	renamed := *bob
	renamed.Email = "Ann@Example.com"
	if err := st.AdminUpdateUser(ctx, &renamed); err == nil {
		t.Error("an admin rename onto an existing address was accepted")
	}
	if got, err := st.GetUser(ctx, bob.ID); err != nil || got.Email != "bob@example.com" {
		t.Errorf("Bob after the refused rename = %+v, %v; want bob@example.com kept", got, err)
	}
}
