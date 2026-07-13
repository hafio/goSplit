package store

import (
	"context"
	"testing"
)

func TestUserThemeColorDefaultAndUpdate(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	// Fresh users default to burgundy (column default applied on insert).
	u, err := st.CreateUser(ctx, &User{Name: "A", Email: "a@x.com"})
	if err != nil {
		t.Fatal(err)
	}
	if u.ThemeColor != "burgundy" {
		t.Fatalf("default theme = %q, want burgundy", u.ThemeColor)
	}

	// UpdateProfile persists a new theme, and a re-read sees it.
	u.ThemeColor = "indigo"
	if err := st.UpdateProfile(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ThemeColor != "indigo" {
		t.Fatalf("after update theme = %q, want indigo", got.ThemeColor)
	}

	// An explicit theme at creation time is honored.
	u2, err := st.CreateUser(ctx, &User{Name: "B", Email: "b@x.com", ThemeColor: "forest"})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ThemeColor != "forest" {
		t.Fatalf("explicit theme = %q, want forest", u2.ThemeColor)
	}
}
