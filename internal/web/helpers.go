package web

import (
	"strings"
	"unicode"
)

// avatarPalette is the fixed set of avatar background colors. avatarColor picks
// one deterministically from an id so a given user or group keeps a stable hue
// across pages. Text on these is always white.
var avatarPalette = []string{
	"#0d9488", "#6366f1", "#d97706", "#e11d48", "#7c3aed", "#0284c7",
}

// avatarColor returns a stable background color for an entity id.
func avatarColor(id int64) string {
	if id < 0 {
		id = -id
	}
	return avatarPalette[id%int64(len(avatarPalette))]
}

// initials derives up to two uppercase initials for an avatar: from the first
// two words of name, falling back to the first two letters of the email
// local-part, then to "?".
func initials(name, email string) string {
	if s := strings.TrimSpace(name); s != "" {
		var rs []rune
		for _, f := range strings.Fields(s) {
			if r := firstAlnum(f); r != 0 {
				rs = append(rs, unicode.ToUpper(r))
				if len(rs) == 2 {
					break
				}
			}
		}
		if len(rs) > 0 {
			return string(rs)
		}
	}
	local := email
	if i := strings.IndexByte(local, '@'); i >= 0 {
		local = local[:i]
	}
	var rs []rune
	for _, r := range strings.TrimSpace(local) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			rs = append(rs, unicode.ToUpper(r))
			if len(rs) == 2 {
				break
			}
		}
	}
	if len(rs) == 0 {
		return "?"
	}
	return string(rs)
}

func firstAlnum(s string) rune {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
	}
	return 0
}

// Category is one selectable expense category with its display emoji.
type Category struct {
	Name  string
	Emoji string
}

// categoryList is the canonical, ordered set offered by the category picker.
// Categories remain free text in the DB; unknown values fall back to the Other
// emoji so imported or legacy categories still render.
var categoryList = []Category{
	{"Food & drink", "🍕"},
	{"Groceries", "🛒"},
	{"Home", "🏠"},
	{"Transport", "🚕"},
	{"Entertainment", "🎬"},
	{"Utilities", "💡"},
	{"Travel", "✈️"},
	{"Health", "🏥"},
	{"Gifts", "🎁"},
	{"Coffee", "☕"},
	{"Fuel", "⛽"},
	{"Other", "📦"},
}

var categoryEmojiByName = func() map[string]string {
	m := make(map[string]string, len(categoryList))
	for _, c := range categoryList {
		m[strings.ToLower(c.Name)] = c.Emoji
	}
	return m
}()

// categoryEmoji maps a stored category name to an emoji, case-insensitively,
// with the Other box as the fallback.
func categoryEmoji(cat string) string {
	if e, ok := categoryEmojiByName[strings.ToLower(strings.TrimSpace(cat))]; ok {
		return e
	}
	return "📦"
}

// categories returns the canonical category list (for the picker).
func categories() []Category { return categoryList }

// fieldErr looks up a per-field validation message. Pages that never set the
// map at all pass an untyped nil here, so it tolerates that rather than forcing
// every render path to remember an empty map -- a missing message must degrade
// to "no message", never to a template error on a page that is already
// reporting a problem.
func fieldErr(m any, key string) string {
	errs, ok := m.(map[string]string)
	if !ok {
		return ""
	}
	return errs[key]
}

// methodGlyph maps a split-method code to the compact glyph shown in the
// segmented control.
func methodGlyph(m string) string {
	switch strings.ToUpper(m) {
	case "EQUAL":
		return "="
	case "EXACT":
		return "1.23"
	case "PERCENTAGE":
		return "%"
	case "SHARE":
		return "⅓"
	case "ADJUSTMENT":
		return "±"
	}
	return m
}

// currencyCodesList is the common ISO set offered by the currency picker; the
// active value is always shown too even if absent here.
var currencyCodesList = []string{
	"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "INR", "CNY",
	"CHF", "SEK", "NZD", "MXN", "BRL", "ZAR", "SGD", "PLN",
}

// currencyCodes returns the common ISO currency codes (for the picker).
func currencyCodes() []string { return currencyCodesList }
