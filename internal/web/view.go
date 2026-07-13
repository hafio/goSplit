// Package web renders server-side HTML using stdlib html/template. Templates
// and static assets are embedded with go:embed so the app ships as one binary.
// The thin client (htmx + a little Alpine) swaps server-rendered fragments.
package web

import (
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/hafio/gosplit/internal/i18n"
	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/store"
)

// csvRows parses a CSV string into rows for table rendering (used to display the
// Historical Transactions note). Returns nil on empty/invalid input.
func csvRows(s string) [][]string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	rows, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		return nil
	}
	return rows
}

// filterKeys is the frozen §5.2 filter query-string contract.
var filterKeys = []string{"q", "min", "max", "from", "to", "scope"}

// queryWithout rebuilds a filter URL (action + query string) from the current
// filter map minus one key — used to render a removable filter chip as a plain
// link, so the filter bar needs no JS. The default scope ("all"/"") is treated
// as unset.
func queryWithout(filter map[string]string, action, drop string) string {
	q := url.Values{}
	for _, k := range filterKeys {
		if k == drop {
			continue
		}
		v := filter[k]
		if v == "" || (k == "scope" && v == "all") {
			continue
		}
		q.Set(k, v)
	}
	if len(q) == 0 {
		return action
	}
	return action + "?" + q.Encode()
}

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

// Renderer holds the parsed page templates and the i18n bundle.
type Renderer struct {
	pages  map[string]*template.Template
	bundle *i18n.Bundle
}

// funcs are template helpers available to every template.
var funcs = template.FuncMap{
	"money":     money.Format,
	"moneyCode": money.FormatWithCode,
	"abs64": func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	},
	"dict": func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[fmt.Sprint(kv[i])] = kv[i+1]
		}
		return m
	},
	"initials":      initials,
	"avatarColor":   avatarColor,
	"categoryEmoji": categoryEmoji,
	"categories":    categories,
	"methodGlyph":   methodGlyph,
	"currencyCodes": currencyCodes,
	"queryWithout":  queryWithout,
	"csvRows":       csvRows,
	"themes":        Themes,
	"themeHex":      themeHex,
}

// pageFiles maps a logical page name to its content template file. Each page is
// parsed together with layout.html and any partials.
var pageFiles = map[string]string{
	"login":          "login.html",
	"register":       "register.html",
	"forgot":         "forgot.html",
	"reset":          "reset.html",
	"message":        "message.html",
	"balances":       "balances.html",
	"friends":        "friends.html",
	"friend":         "friend.html",
	"groups":         "groups.html",
	"group":          "group.html",
	"expense_form":   "expense_form.html",
	"expense_detail": "expense_detail.html",
	"activity":       "activity.html",
	"profile":        "profile.html",
	"admin":          "admin.html",
	"recurring":      "recurring.html",
	"convert":        "convert.html",
	"import":         "import.html",
	"bank":           "bank.html",
	"collapse":       "collapse.html",
}

// NewRenderer parses all page templates once at startup and loads i18n.
func NewRenderer() (*Renderer, error) {
	bundle, err := i18n.Load()
	if err != nil {
		return nil, fmt.Errorf("web: load i18n: %w", err)
	}
	r := &Renderer{pages: map[string]*template.Template{}, bundle: bundle}
	for name, file := range pageFiles {
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(templatesFS,
			"templates/layout.html", "templates/partials.html", "templates/"+file)
		if err != nil {
			return nil, fmt.Errorf("web: parse %s: %w", file, err)
		}
		r.pages[name] = t
	}
	return r, nil
}

// DetectLang resolves the best locale for a request (user preference, then
// Accept-Language, then default).
func (r *Renderer) DetectLang(userPref, acceptLanguage string) string {
	return r.bundle.Detect(userPref, acceptLanguage)
}

// Languages returns the available locale codes (for the profile selector).
func (r *Renderer) Languages() []string { return r.bundle.Languages() }

// T translates a key for lang directly (for use outside template rendering,
// e.g. page titles and flash messages built server-side).
func (r *Renderer) T(lang, key string) string { return r.bundle.T(lang, key) }

// ViewData wraps page-specific data with the ambient user/CSRF/flash context.
type ViewData struct {
	Title  string
	User   *store.User
	CSRF   string
	Flash  string
	Error  string
	Lang   string
	Theme  string // accent theme slug; defaults to DefaultTheme
	Nav    string // active primary-nav slug (balances/friends/groups/activity)
	Data   any
	bundle *i18n.Bundle
}

// T translates a key in the view's language (falls back to the key itself).
func (v ViewData) T(key string) string {
	if v.bundle == nil {
		return key
	}
	return v.bundle.T(v.Lang, key)
}

// Render writes a full page (executing the layout).
func (r *Renderer) Render(w http.ResponseWriter, status int, page string, vd ViewData) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	vd.bundle = r.bundle
	if vd.Lang == "" {
		vd.Lang = i18n.DefaultLang
	}
	if vd.Theme == "" || !ValidTheme(vd.Theme) {
		vd.Theme = DefaultTheme
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "layout.html", vd); err != nil {
		// Header already written; log-and-continue is the best we can do.
		fmt.Fprintf(io.Discard, "render error: %v", err)
	}
}

// AssetsHandler serves embedded static assets under /static/.
func (r *Renderer) AssetsHandler() http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

// Asset returns a single embedded asset's bytes (for manifest/service worker
// served at the root).
func Asset(name string) ([]byte, error) {
	return assetsFS.ReadFile("assets/" + name)
}
