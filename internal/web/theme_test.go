package web

import "testing"

func TestValidTheme(t *testing.T) {
	for _, ok := range []string{"burgundy", "teal", "indigo", "forest", "amber", "slate"} {
		if !ValidTheme(ok) {
			t.Errorf("ValidTheme(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "lime", "Burgundy", "<script>", "red"} {
		if ValidTheme(bad) {
			t.Errorf("ValidTheme(%q) = true, want false", bad)
		}
	}
}

func TestDefaultThemeIsValid(t *testing.T) {
	if !ValidTheme(DefaultTheme) {
		t.Fatalf("DefaultTheme %q is not a registered theme", DefaultTheme)
	}
	if DefaultTheme != "burgundy" {
		t.Errorf("DefaultTheme = %q, want burgundy", DefaultTheme)
	}
}

func TestThemeHex(t *testing.T) {
	if got := themeHex("teal", false); got != "#0f766e" {
		t.Errorf("themeHex(teal,light) = %q", got)
	}
	if got := themeHex("teal", true); got != "#2dd4bf" {
		t.Errorf("themeHex(teal,dark) = %q", got)
	}
	// unknown slug falls back to the burgundy default in both modes
	if got := themeHex("nope", false); got != "#8d3f4c" {
		t.Errorf("themeHex(unknown,light) = %q, want burgundy", got)
	}
	if got := themeHex("nope", true); got != "#d98d98" {
		t.Errorf("themeHex(unknown,dark) = %q, want burgundy dark", got)
	}
}

func TestThemesComplete(t *testing.T) {
	for _, th := range Themes() {
		if th.Slug == "" || th.Label == "" || th.LightAccent == "" || th.DarkAccent == "" ||
			th.LightInk == "" || th.DarkInk == "" || th.LightSoft == "" || th.DarkSoft == "" {
			t.Errorf("incomplete theme: %+v", th)
		}
	}
}
