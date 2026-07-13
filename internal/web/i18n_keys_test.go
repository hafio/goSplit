package web

import (
	"io/fs"
	"regexp"
	"testing"

	"github.com/hafio/gosplit/internal/i18n"
)

// keyRe matches static .T "key" calls in templates (dynamic printf-built keys,
// e.g. (printf "theme.%s" .Slug), are intentionally not matched here).
var keyRe = regexp.MustCompile(`\.T "([a-zA-Z0-9_.]+)"`)

// TestTemplateKeysResolve walks every embedded template and asserts each
// static .T "key" reference resolves to a real (non-fallback) value in the
// default locale, catching typos and stale keys at build time.
func TestTemplateKeysResolve(t *testing.T) {
	bundle, err := i18n.Load()
	if err != nil {
		t.Fatalf("load i18n: %v", err)
	}

	err = fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range keyRe.FindAllStringSubmatch(string(data), -1) {
			key := m[1]
			if got := bundle.T(i18n.DefaultLang, key); got == key {
				t.Errorf("%s: key %q not found in default locale %q", path, key, i18n.DefaultLang)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
}
