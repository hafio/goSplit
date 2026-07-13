// Package i18n provides message catalogs with per-user language selection and
// Accept-Language auto-detection. Catalogs are flat JSON key→string files
// (Weblate-compatible) embedded at build time; adding a locale needs no code
// change — just drop in locales/<lang>.json.
package i18n

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

// DefaultLang is the fallback locale.
const DefaultLang = "en"

// Bundle holds all loaded catalogs.
type Bundle struct {
	catalogs map[string]map[string]string
	langs    []string
}

// Load reads every embedded locale file into a Bundle.
func Load() (*Bundle, error) {
	b := &Bundle{catalogs: map[string]map[string]string{}}
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := localesFS.ReadFile("locales/" + e.Name())
		if err != nil {
			return nil, err
		}
		var cat map[string]string
		if err := json.Unmarshal(data, &cat); err != nil {
			return nil, err
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		b.catalogs[lang] = cat
		b.langs = append(b.langs, lang)
	}
	sort.Strings(b.langs)
	return b, nil
}

// Languages returns the available locale codes.
func (b *Bundle) Languages() []string { return b.langs }

// Has reports whether a locale is loaded.
func (b *Bundle) Has(lang string) bool { _, ok := b.catalogs[lang]; return ok }

// T translates key for lang, falling back to the default locale and then to the
// key itself (so a missing translation is visible but never blank).
func (b *Bundle) T(lang, key string) string {
	if cat, ok := b.catalogs[lang]; ok {
		if v, ok := cat[key]; ok && v != "" {
			return v
		}
	}
	if cat, ok := b.catalogs[DefaultLang]; ok {
		if v, ok := cat[key]; ok && v != "" {
			return v
		}
	}
	return key
}

// Detect picks the best available locale: the user's explicit preference if
// loaded, else the first matching Accept-Language tag, else the default.
func (b *Bundle) Detect(userPref, acceptLanguage string) string {
	if userPref != "" && b.Has(userPref) {
		return userPref
	}
	for _, tag := range parseAcceptLanguage(acceptLanguage) {
		if b.Has(tag) {
			return tag
		}
		// Try the primary subtag (e.g. "en-US" -> "en").
		if i := strings.IndexByte(tag, '-'); i > 0 && b.Has(tag[:i]) {
			return tag[:i]
		}
	}
	return DefaultLang
}

// parseAcceptLanguage returns language tags ordered by descending q-value.
func parseAcceptLanguage(header string) []string {
	type qtag struct {
		tag string
		q   float64
	}
	var tags []qtag
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tag, q := part, 1.0
		if i := strings.Index(part, ";q="); i >= 0 {
			tag = strings.TrimSpace(part[:i])
			q = parseQ(part[i+3:])
		}
		tags = append(tags, qtag{strings.ToLower(tag), q})
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].q > tags[j].q })
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.tag
	}
	return out
}

func parseQ(s string) float64 {
	s = strings.TrimSpace(s)
	switch s {
	case "", "1", "1.0":
		return 1.0
	case "0", "0.0":
		return 0.0
	}
	// crude decimal parse without strconv overhead concerns
	var whole, frac float64
	var div float64 = 1
	seenDot := false
	for _, c := range s {
		if c == '.' {
			seenDot = true
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		if seenDot {
			div *= 10
			frac += float64(c-'0') / div
		} else {
			whole = whole*10 + float64(c-'0')
		}
	}
	return whole + frac
}
