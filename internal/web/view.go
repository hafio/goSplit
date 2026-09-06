// Package web renders server-side HTML using stdlib html/template. Templates
// and static assets are embedded with go:embed so the app ships as one binary.
// The thin client is htmx: navigation is boosted and morphed into the existing
// body (Render), while requests that name a region via HX-Target get just that
// region's block (RenderFragment). Both paths execute the same templates, so a
// fragment can never drift from the full page it came from.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

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
var filterKeys = []string{"q", "min", "max", "from", "to", "scope", "archived"}

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

// assetHashes maps asset filename → short content hash, computed once at
// startup so templates can emit fingerprinted, immutable-cacheable URLs.
var assetHashes = func() map[string]string {
	m := map[string]string{}
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return m
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := assetsFS.ReadFile("assets/" + e.Name())
		if err != nil {
			continue
		}
		sum := sha256.Sum256(b)
		m[e.Name()] = hex.EncodeToString(sum[:5])
	}
	return m
}()

const staticPrefix = "/static/"

// assetURL returns the fingerprinted /static URL for an embedded asset.
func assetURL(name string) string {
	if h, ok := assetHashes[name]; ok {
		return staticPrefix + name + "?v=" + h
	}
	return staticPrefix + name
}

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
	"fieldErr":      fieldErr,
	"themes":        Themes,
	"themeHex":      themeHex,
	"asset":         assetURL,
	"pollable":      pollable,
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
	"admin_backup":   "admin_backup.html",
	"recurring":      "recurring.html",
	"convert":        "convert.html",
	"import":         "import.html",
	"bank":           "bank.html",
	"collapse":       "collapse.html",
	"settle_group":   "settle_group.html",
}

// Fragment names a block of a page that htmx may swap on its own, keyed by the
// DOM id it lives in. Poll marks a region safe to refresh on a timer.
type Fragment struct {
	Block string
	Poll  bool
}

// contentFragment is every page's whole content block. It is what the freshness
// poller re-requests, so a page becomes pollable by adding one entry here -- no
// per-page fragment markup required.
var contentFragment = Fragment{Block: "content", Poll: true}

// fragments maps page -> HX-Target id -> the block that answers it. Only list
// and detail pages are pollable: re-rendering a form under someone would throw
// away what they had typed, so expense_form, convert, settle_group, collapse,
// profile, admin, import, bank and the auth pages are deliberately absent.
var fragments = map[string]map[string]Fragment{
	"balances":       {"content": contentFragment},
	"friends":        {"content": contentFragment},
	"friend":         {"content": contentFragment, "friend-feed": {Block: "frag_friend_feed"}},
	"groups":         {"content": contentFragment},
	"group":          {"content": contentFragment, "group-feed": {Block: "frag_group_feed"}},
	"activity":       {"content": contentFragment, "activity-feed": {Block: "frag_activity_feed"}},
	"expense_detail": {"content": contentFragment},
	"recurring":      {"content": contentFragment},
}

// FragmentFor returns the block registered for a page's swap target.
func FragmentFor(page, target string) (Fragment, bool) {
	f, ok := fragments[page][target]
	return f, ok
}

// pollable reports whether a page's content may be refreshed on a timer; the
// layout uses it to decide whether to emit the poller at all.
func pollable(page string) bool { return fragments[page]["content"].Poll }

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
	// A registry entry naming a page or block that does not exist is a wiring
	// mistake that would otherwise surface as a 500 the first time a user
	// happened to trigger that swap. Fail at startup instead.
	if err := validateFragments(r.pages, fragments); err != nil {
		return nil, err
	}
	return r, nil
}

// validateFragments takes the registry rather than reading the package one, so
// the failure paths are testable without mutating shared state.
func validateFragments(pages map[string]*template.Template, reg map[string]map[string]Fragment) error {
	for page, byTarget := range reg {
		t, ok := pages[page]
		if !ok {
			return fmt.Errorf("web: fragment registry names unknown page %q", page)
		}
		for target, f := range byTarget {
			if t.Lookup(f.Block) == nil {
				return fmt.Errorf("web: page %q target %q names unknown block %q", page, target, f.Block)
			}
		}
	}
	return nil
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
	Title string
	User  *store.User
	CSRF  string
	Flash string
	Error string
	Lang  string
	Theme string // accent theme slug; defaults to DefaultTheme
	Nav   string // active primary-nav slug (balances/friends/groups/activity)
	// Version is the build tag stamped into the binary, shown in the footer so
	// it is possible to tell which build a running container is serving.
	Version string
	// Page is the logical page name, so the layout can ask whether this page's
	// content may be refreshed on a timer. Path is the URL the poller re-asks
	// for.
	Page string
	Path string
	Data any

	bundle *i18n.Bundle
}

// T translates a key in the view's language (falls back to the key itself).
func (v ViewData) T(key string) string {
	if v.bundle == nil {
		return key
	}
	return v.bundle.T(v.Lang, key)
}

// resolve fills in the ambient defaults a template expects.
func (r *Renderer) resolve(vd *ViewData) {
	vd.bundle = r.bundle
	if vd.Lang == "" {
		vd.Lang = i18n.DefaultLang
	}
	if vd.Theme == "" || !ValidTheme(vd.Theme) {
		vd.Theme = DefaultTheme
	}
}

// htmlHeaders sets the headers every HTML response shares.
func htmlHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// Pages are always served fresh from the server; never cached client-side
	// (static assets keep their own long-lived caching in AssetsHandler).
	h.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
}

// untimedPages get no Server-Timing header: how long a credential check took is
// not something to publish.
var untimedPages = map[string]bool{
	"login": true, "register": true, "forgot": true, "reset": true, "message": true,
}

// setServerTiming publishes render cost so a page load doubles as a profiling
// sample, which is what tells us whether query work is worth tuning at all.
func setServerTiming(w http.ResponseWriter, page string, d time.Duration) {
	if untimedPages[page] {
		return
	}
	w.Header().Set("Server-Timing", fmt.Sprintf("render;dur=%.1f", float64(d.Microseconds())/1000))
}

// execute renders a block into memory and reports how long it took. Building
// the response before committing a status is what lets both render paths report
// a template failure instead of emitting a half-written page.
func (r *Renderer) execute(t *template.Template, block string, vd ViewData) ([]byte, time.Duration, error) {
	start := time.Now()
	var buf bytes.Buffer
	err := t.ExecuteTemplate(&buf, block, vd)
	return buf.Bytes(), time.Since(start), err
}

// Render writes a full page (executing the layout).
func (r *Renderer) Render(w http.ResponseWriter, status int, page string, vd ViewData) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	r.resolve(&vd)
	out, dur, err := r.execute(t, "layout.html", vd)
	if err != nil {
		http.Error(w, "render error: "+page, http.StatusInternalServerError)
		return
	}
	htmlHeaders(w)
	setServerTiming(w, page, dur)
	w.WriteHeader(status)
	if _, err := w.Write(out); err != nil {
		fmt.Fprintf(io.Discard, "page write error: %v", err)
	}
}

// FragmentVersionHeader carries the fingerprint of the block just rendered. The
// client echoes it back as ?v= on its next poll.
const FragmentVersionHeader = "X-Fragment-Version"

// RenderFragment writes one named block of a page instead of the whole layout,
// for an htmx request that targets a single region. block must be defined in
// that page's template set; an unknown page or block is a programming error and
// fails loudly rather than silently returning an empty body.
//
// The block is rendered into a buffer and fingerprinted so an unchanged region
// can be answered with 204 and no body: that is what lets the freshness poller
// run every few seconds without re-sending a page that has not moved. Hashing
// the rendered output rather than a stored updated_at is deliberate -- it is
// correct by construction for anything the template shows (a renamed friend, a
// changed member list), with no schema to keep in step. It does assume the
// block has no per-request noise in it; the double-submit CSRF token is
// per-session, so a form inside a polled page does not defeat it.
func (r *Renderer) RenderFragment(w http.ResponseWriter, req *http.Request, status int, page, block string, vd ViewData) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	if t.Lookup(block) == nil {
		http.Error(w, "unknown fragment: "+page+"/"+block, http.StatusInternalServerError)
		return
	}
	r.resolve(&vd)
	out, dur, err := r.execute(t, block, vd)
	if err != nil {
		http.Error(w, "fragment render error: "+page+"/"+block, http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(out)
	version := hex.EncodeToString(sum[:8])

	htmlHeaders(w)
	setServerTiming(w, page, dur)
	w.Header().Set(FragmentVersionHeader, version)
	if req != nil && req.URL.Query().Get("v") == version {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(status)
	if _, err := w.Write(out); err != nil {
		fmt.Fprintf(io.Discard, "fragment write error: %v", err)
	}
}

// AssetsHandler serves embedded static assets under /static/. Fingerprinted
// URLs (?v=<hash>) are immutable; unversioned ones get a short cache.
func (r *Renderer) AssetsHandler() http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix(staticPrefix, http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fileServer.ServeHTTP(w, req)
	})
}

// Asset returns a single embedded asset's bytes (for manifest/service worker
// served at the root).
func Asset(name string) ([]byte, error) {
	return assetsFS.ReadFile("assets/" + name)
}
