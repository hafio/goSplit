package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("unexpected hash format: %s", h)
	}
	if err := VerifyPassword("correct horse battery staple", h); err != nil {
		t.Fatalf("verify should succeed: %v", err)
	}
	if err := VerifyPassword("wrong", h); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want ErrMismatch, got %v", err)
	}
}

func TestHashPassword_Empty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("empty password should error")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	bad := []string{
		"not-a-hash",
		"$argon2i$v=19$m=1,t=1,p=1$aaaa$bbbb",
		"$argon2id$vX$m=1,t=1,p=1$aaaa$bbbb",
		"$argon2id$v=19$bad$aaaa$bbbb",
		"$argon2id$v=19$m=1,t=1,p=1$!!!$bbbb",
		"$argon2id$v=19$m=1,t=1,p=1$YWJj$!!!",
	}
	for _, b := range bad {
		if err := VerifyPassword("x", b); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("VerifyPassword(%q) = %v, want ErrInvalidHash", b, err)
		}
	}
}

func TestRandomTokenUnique(t *testing.T) {
	a, b := RandomToken(32), RandomToken(32)
	if a == b {
		t.Fatal("tokens should be unique")
	}
	if len(a) == 0 {
		t.Fatal("token empty")
	}
}
