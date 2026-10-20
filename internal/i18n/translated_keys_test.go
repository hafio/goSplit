package i18n

import "testing"

// translatedKeys are keys that must carry a real value in every locale, not
// just in the default one. Missing translations are tolerated in general (T
// falls back to English), but these were added with all nine translations and
// are pinned here so a later edit cannot silently drop one. It also covers keys
// used only from Go (s.tr), which the template key test never sees.
var translatedKeys = []string{
	// Balances page note.
	"balances.not_simplified", "balances.simplified_link",
	// LIMITED role.
	"msg.limited_title", "msg.limited_body", "admin.role_hint",
	// Admin activity feed and admin transaction powers.
	"title.admin_activity", "admin.activity_hint", "admin.filter_user", "admin.filter_group",
	"admin.all_users", "admin.all_groups", "detail.admin_override", "detail.deleted_by",
	// Admin view of any group.
	"title.admin_groups", "group.admin_view", "admin.groups_empty",
	// Netting entries and the tidy-up.
	"feed.netting", "feed.netting_sub", "detail.netting_explain", "err.netting_immutable",
	"group.tidy", "group.tidy_hint", "msg.tidy_failed", "settle.all_intro",
}

// TestTranslatedKeysInEveryLocale fails when a pinned key is missing or empty
// in any locale, including the default. A new locale file therefore needs these
// keys too (copying en.json is enough to start).
func TestTranslatedKeysInEveryLocale(t *testing.T) {
	b := load(t)
	if !b.Has(DefaultLang) {
		t.Fatalf("default locale %q not loaded", DefaultLang)
	}
	for _, lang := range b.Languages() {
		for _, k := range translatedKeys {
			if v := b.catalogs[lang][k]; v == "" {
				t.Errorf("locale %q has no value for %q", lang, k)
			}
		}
	}
}
