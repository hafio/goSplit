package i18n

import (
	"sort"
	"testing"
)

func load(t *testing.T) *Bundle {
	t.Helper()
	b, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return b
}

// firstOther returns any loaded locale other than the default, or "".
func firstOther(b *Bundle) string {
	for _, l := range b.Languages() {
		if l != DefaultLang {
			return l
		}
	}
	return ""
}

// coreKey is a key expected in the default catalog; used only to probe behavior,
// read dynamically so the test doesn't hardcode a translated value.
const coreKey = "nav.balances"

func TestTranslate(t *testing.T) {
	b := load(t)

	base := b.T(DefaultLang, coreKey)
	if base == "" || base == coreKey {
		t.Fatalf("default locale %q missing %q (got %q)", DefaultLang, coreKey, base)
	}
	// Missing key → the key itself (visible, never blank).
	if got := b.T(DefaultLang, "definitely.missing.key"); got != "definitely.missing.key" {
		t.Fatalf("missing key fallback = %q", got)
	}
	// An unknown language falls back to the default locale's value.
	if got := b.T("zz", coreKey); got != base {
		t.Fatalf("unknown-lang fallback = %q, want %q", got, base)
	}
	// Every loaded catalog returns its own stored value for its own keys.
	for _, lang := range b.Languages() {
		for k, v := range b.catalogs[lang] {
			if v == "" {
				continue
			}
			if got := b.T(lang, k); got != v {
				t.Errorf("T(%q,%q)=%q want %q", lang, k, got, v)
			}
			break // one representative key per locale is enough
		}
	}
}

func TestDetect(t *testing.T) {
	b := load(t)

	// A code that must not correspond to any catalog. Guard in case one is added.
	const unsupported = "qq"
	if b.Has(unsupported) {
		t.Skipf("test assumes %q is not a loaded locale", unsupported)
	}

	type dcase struct{ pref, accept, want string }
	cases := []dcase{
		{"", "", DefaultLang},                    // nothing → default
		{unsupported, "", DefaultLang},           // unknown preference → default
		{DefaultLang, "", DefaultLang},           // preference is loaded
		{unsupported, DefaultLang, DefaultLang},  // bad preference, Accept-Language wins
		{"", DefaultLang + "-XX;q=0.9", DefaultLang}, // region variant → primary subtag
		{"", unsupported + "-ZZ," + unsupported + ";q=0.9", DefaultLang}, // unknown Accept → default
	}
	// Exercise a real non-default locale when one exists, without naming it.
	if other := firstOther(b); other != "" {
		cases = append(cases,
			dcase{other, "", other},          // explicit preference
			dcase{"", other, other},          // Accept-Language exact match
			dcase{unsupported, other, other}, // bad preference falls through to Accept
		)
	}
	for _, c := range cases {
		if got := b.Detect(c.pref, c.accept); got != c.want {
			t.Errorf("Detect(%q,%q)=%q want %q", c.pref, c.accept, got, c.want)
		}
	}
}

func TestLanguages(t *testing.T) {
	b := load(t)
	if len(b.Languages()) == 0 {
		t.Fatal("no locales loaded")
	}
	if !b.Has(DefaultLang) {
		t.Fatalf("default locale %q not loaded; have %v", DefaultLang, b.Languages())
	}
	if !sort.StringsAreSorted(b.Languages()) {
		t.Errorf("Languages() should be sorted, got %v", b.Languages())
	}
}

// TestNoOrphanTranslationKeys catches keys present in a translated catalog but
// absent from the default one (usually a typo) — it never fires on partial
// translations, since those are keys the default still has.
func TestNoOrphanTranslationKeys(t *testing.T) {
	b := load(t)
	base := b.catalogs[DefaultLang]
	for _, lang := range b.Languages() {
		if lang == DefaultLang {
			continue
		}
		for k := range b.catalogs[lang] {
			if _, ok := base[k]; !ok {
				t.Errorf("locale %q has key %q not present in default %q catalog", lang, k, DefaultLang)
			}
		}
	}
}
