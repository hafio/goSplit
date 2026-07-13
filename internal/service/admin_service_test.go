package service

import (
	"context"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/store"
)

func TestAdminCreateUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// Password-less account: NULL hash, currency/lang defaults applied.
	u, err := svc.AdminCreateUser(ctx, "Ann", "ANN@x.com", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ann@x.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.Role != "USER" || u.Currency != "USD" || u.PreferredLanguage != "en" {
		t.Errorf("defaults not applied: %+v", u)
	}
	if u.PasswordHash.Valid {
		t.Errorf("password-less account should have NULL hash")
	}

	// With a password: login works.
	if _, err := svc.AdminCreateUser(ctx, "Bo", "bo@x.com", "ADMIN", "EUR", "es", "password12"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LoginPassword(ctx, "bo@x.com", "password12"); err != nil {
		t.Errorf("login with set password failed: %v", err)
	}

	// Duplicate email, bad role, short password all rejected.
	if _, err := svc.AdminCreateUser(ctx, "", "bo@x.com", "USER", "", "", ""); err != ErrEmailTaken {
		t.Errorf("dup email: got %v", err)
	}
	if _, err := svc.AdminCreateUser(ctx, "", "x@x.com", "SUPER", "", "", ""); err != ErrInvalidRole {
		t.Errorf("bad role: got %v", err)
	}
	if _, err := svc.AdminCreateUser(ctx, "", "y@x.com", "USER", "", "", "short"); err != ErrPasswordTooShort {
		t.Errorf("short password: got %v", err)
	}
	if _, err := svc.AdminCreateUser(ctx, "", "  ", "USER", "", "", ""); err != ErrEmailRequired {
		t.Errorf("blank email: got %v", err)
	}
}

func TestAdminSetPasswordRevokesSessions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u, _ := svc.Store.CreateUser(ctx, &store.User{Name: "A", Email: "a@x.com"})
	if _, err := svc.Store.CreateSession(ctx, "tok-1", u.ID, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.AdminSetPassword(ctx, u.ID, "short"); err != ErrPasswordTooShort {
		t.Fatalf("short password: got %v", err)
	}
	if err := svc.AdminSetPassword(ctx, u.ID, "newpassword12"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.GetSession(ctx, "tok-1"); err != store.ErrNotFound {
		t.Errorf("session not revoked: %v", err)
	}
	if _, err := svc.LoginPassword(ctx, "a@x.com", "newpassword12"); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
}
