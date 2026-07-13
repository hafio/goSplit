package web

import "testing"

func TestInitials(t *testing.T) {
	cases := []struct {
		name, email, want string
	}{
		{"Priya Sharma", "p@x.com", "PS"},
		{"marcus", "m@x.com", "M"},
		{"  Elena   Kaur  ", "e@x.com", "EK"},
		{"Jean-Luc Picard", "j@x.com", "JP"},
		{"", "alice@example.com", "AL"},
		{"", "42robot@example.com", "42"},
		{"", "", "?"},
		{"   ", "  ", "?"},
		{"李 明", "l@x.com", "李明"},
	}
	for _, c := range cases {
		if got := initials(c.name, c.email); got != c.want {
			t.Errorf("initials(%q,%q)=%q want %q", c.name, c.email, got, c.want)
		}
	}
}

func TestAvatarColorStableAndInRange(t *testing.T) {
	if avatarColor(3) != avatarColor(3) {
		t.Fatal("avatarColor not deterministic")
	}
	// negative ids must not panic and must land in the palette
	seen := map[string]bool{}
	for _, id := range []int64{0, 1, 2, 3, 4, 5, 6, -7, 100} {
		c := avatarColor(id)
		found := false
		for _, p := range avatarPalette {
			if p == c {
				found = true
			}
		}
		if !found {
			t.Errorf("avatarColor(%d)=%q not in palette", id, c)
		}
		seen[c] = true
	}
	if len(seen) < 2 {
		t.Error("expected avatarColor to spread across the palette")
	}
}

func TestCategoryEmoji(t *testing.T) {
	cases := map[string]string{
		"Food & drink": "🍕",
		"food & drink": "🍕", // case-insensitive
		"  Coffee ":    "☕", // trims
		"Groceries":    "🛒",
		"Rent":         "📦", // unknown → Other
		"":             "📦",
	}
	for in, want := range cases {
		if got := categoryEmoji(in); got != want {
			t.Errorf("categoryEmoji(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCategoriesNonEmpty(t *testing.T) {
	cs := categories()
	if len(cs) == 0 {
		t.Fatal("categories() empty")
	}
	for _, c := range cs {
		if c.Name == "" || c.Emoji == "" {
			t.Errorf("category with empty field: %+v", c)
		}
		if categoryEmoji(c.Name) != c.Emoji {
			t.Errorf("categoryEmoji(%q) disagrees with list emoji %q", c.Name, c.Emoji)
		}
	}
}
