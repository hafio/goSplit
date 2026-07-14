# GoSplit Performance Plan — fast page loads, instant clicks

## Context

Every page navigation feels slow. Investigation traced it to compounding causes — most time is wasted on the network/client side, plus server-side costs that grow with data:

1. **No static-asset caching.** `AssetsHandler` (internal/web/view.go:202) serves `embed.FS` files with no `Cache-Control`; embedded files have no modtime so no 304 path exists. The browser re-downloads `app.css`, `icons.svg`, `favicon.svg`, `htmx.min.js` on **every navigation**.
2. **Service worker defeats its cache.** `internal/web/assets/sw.js` is network-first for everything — the cache is only used offline; online it adds latency and saves nothing.
3. **Static requests pay for auth.** `Authenticate` (2 SQLite queries) runs before `/static/*` (internal/httpapp/server.go:45–49) and the store uses `SetMaxOpenConns(1)` — every asset request serializes 2 DB queries on one connection.
4. **htmx is a placeholder.** `internal/web/assets/htmx.min.js` is a 353-byte comment stub. Every click is a full page teardown/re-parse.
5. **No response compression.** Bare Go server, no gzip.
6. **DB amplification.** `balance_view` re-aggregates ALL expenses per query; `/friends` calls it once per friend (N+1); `/groups` runs 2 queries per group; expense feeds have no `LIMIT`.

User approved: real htmx + `hx-boost`, and feed limits with "Show all".

---

## Executor rules (read first)

- **Never run build/vet/test/coverage yourself.** After each phase, ask the user to run `./scripts/dev.ps1 all` (PowerShell) or `./scripts/dev.sh all` (bash) and continue from the output they report. (Project CLAUDE.md rule.)
- Downloading the htmx asset file with curl (Phase B) is allowed — it is not a verification gate.
- **Step 0:** copy this plan to `docs/perf-plan.md` (project convention: plans live in `docs/*-plan.md`) and maintain the **Ledger** at the bottom there — after each phase record what actually shipped (files, decisions).
- Do the phases in order: A (caching), B (htmx), C (DB). Each phase compiles and passes tests on its own.
- Read each file before editing. Match surrounding code style and comment density.
- At the very end, ask the user to run `python -m graphify update .`.

---

## Phase A — Asset caching, compression, auth-free static routes

### A1. Fingerprinted asset URLs — `internal/web/view.go`

Add imports `crypto/sha256`, `encoding/hex`. Below the `//go:embed assets/*` block, add:

```go
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

// assetURL returns the fingerprinted /static URL for an embedded asset.
func assetURL(name string) string {
	if h, ok := assetHashes[name]; ok {
		return "/static/" + name + "?v=" + h
	}
	return "/static/" + name
}
```

Register it in the existing `funcs` map (view.go:73): add `"asset": assetURL,`.

Replace `AssetsHandler` (view.go:202–208) with a version that sets cache headers — fingerprinted requests are immutable, others get 1 hour:

```go
// AssetsHandler serves embedded static assets under /static/. Fingerprinted
// URLs (?v=<hash>) are immutable; unversioned ones get a short cache.
func (r *Renderer) AssetsHandler() http.Handler {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fileServer.ServeHTTP(w, req)
	})
}
```

### A2. Templates use `{{asset}}` — 14 files in `internal/web/templates/`

Find every occurrence: grep `/static/` in `internal/web/templates/*.html` (57 hits). Rewrite pattern:

- `href="/static/app.css"` → `href="{{asset "app.css"}}"`
- `src="/static/htmx.min.js"` → `src="{{asset "htmx.min.js"}}"`
- sprite refs: `<use href="/static/icons.svg#wallet"/>` → `<use href="{{asset "icons.svg"}}#wallet"/>` (query + fragment is a valid URL for `<use>`)
- same for `favicon.svg`, `expense_form.js`, `convert.js`, `push.js`, `bank.js` wherever referenced.

Files: layout.html (14), partials.html (3), group.html (8), friend.html (7), admin.html (6), convert.html (4), expense_form.html (4), collapse.html (2), expense_detail.html (2), friends.html (2), groups.html (2), balances.html (1), bank.html (1), profile.html (1).

Do NOT touch `/static/` strings inside `sw.js` (its SHELL list stays unversioned on purpose) or inside Go code.

### A3. Router: compression + static routes outside auth — `internal/httpapp/server.go`

Rewrite the top of `Router()` (lines 40–54). Global middleware, then public asset routes, then wrap **everything else** (auth pages, `/`, `/offline`, authenticated group, admin group — all unchanged) in a `r.Group` that carries `Authenticate` + `VerifyCSRF`:

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
r.Use(requestLogger)
r.Use(middleware.Recoverer)
r.Use(middleware.Compress(5))

// Public assets + health: no session lookup, no CSRF, cacheable.
r.Handle("/static/*", s.Renderer.AssetsHandler())
r.Handle("/uploads/*", cacheControl("public, max-age=3600",
	http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.Cfg.UploadDir)))))
r.Get("/healthz", s.handleHealth)
r.Get("/manifest.webmanifest", s.serveAsset("manifest.webmanifest", "application/manifest+json", "public, max-age=3600"))
r.Get("/sw.js", s.serveAsset("sw.js", "application/javascript", "no-cache"))

r.Group(func(r chi.Router) {
	r.Use(s.Auth.Authenticate)
	r.Use(s.Auth.VerifyCSRF)
	r.Get("/offline", s.handleOffline)
	// ... all existing auth routes, "/", authenticated r.Group, admin r.Group
	//     move here VERBATIM (bodies unchanged).
})
```

Notes:
- `/offline` moves inside the group (it renders ViewData and wants the user/CSRF context).
- chi rule: `r.Use` must precede route registration on the same router — the `r.Group` closure is the correct way to scope middleware.
- `middleware.Compress(5)` is already available in `github.com/go-chi/chi/v5/middleware`; its default type list covers html/css/js/json/svg. No new dependency.

Update `serveAsset` (server.go:234) to take a cache-control value:

```go
func (s *Server) serveAsset(name, contentType, cache string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := web.Asset(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cache)
		_, _ = w.Write(b)
	}
}
```

Add next to it:

```go
// cacheControl sets a Cache-Control header before delegating to next.
func cacheControl(v string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", v)
		next.ServeHTTP(w, r)
	})
}
```

### A4. Service worker: cache-first for /static — `internal/web/assets/sw.js`

- Change `const CACHE = 'gosplit-v2';` → `'gosplit-v3'` (mandatory — purges old caches on activate).
- Replace the `fetch` listener's non-navigation branch so `/static/*` is cache-first (safe because URLs are now fingerprinted); everything else keeps today's network-first behavior:

```js
self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  if (req.mode === 'navigate') {
    e.respondWith(fetch(req).catch(() => caches.match('/offline')));
    return;
  }
  const url = new URL(req.url);
  if (url.origin === location.origin && url.pathname.startsWith('/static/')) {
    // Fingerprinted static assets: cache-first (URL changes when content does).
    e.respondWith(
      caches.match(req).then((hit) => hit || fetch(req).then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
        return res;
      }).catch(() => caches.match(req, { ignoreSearch: true }))),
    );
    return;
  }
  // Other GETs (e.g. /uploads): network-first with cache fallback (unchanged).
  e.respondWith(
    fetch(req).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
      return res;
    }).catch(() => caches.match(req)),
  );
});
```

Also update the top-of-file comment (it currently documents network-first-for-everything).

### A5. Tests for Phase A

- `internal/web/helpers_test.go` — add `TestAssetURLFingerprinted`: `assetURL("app.css")` starts with `/static/app.css?v=` and is stable across calls; `assetURL("nope.css")` returns `/static/nope.css`.
- `internal/httpapp/http_test.go` (uses the existing `newHarness`) — add `TestStaticCacheHeaders`: GET `/static/app.css?v=x` → `Cache-Control` contains `immutable`; GET `/static/app.css` → contains `max-age=3600`; GET `/sw.js` → `no-cache`.
- Existing `TestTemplateKeysResolve` (web package) will catch any typo in the `{{asset}}` rewrites when the user runs the test gate.

**Checkpoint:** ask the user to run `./scripts/dev.ps1 all` and report results before starting Phase B.

---

## Phase B — Real htmx + hx-boost (instant clicks)

### B1. Vendor htmx

Replace the placeholder by downloading the real build (~50KB min):

```
curl -L -o internal/web/assets/htmx.min.js https://unpkg.com/htmx.org@2/dist/htmx.min.js
```

Verify the file now starts with minified JS (contains a `htmx.org` version string, not the old placeholder comment) and is roughly 50KB. Record the exact resolved version (visible in the file's first line) in the Ledger and README.

### B2. Enable boost — `internal/web/templates/layout.html`

- Change `<body>` (line 14) to:

```html
<body hx-boost="true" hx-indicator="body">
```

  With this, every link/form does an AJAX body swap (no CSS/JS re-parse), htmx updates `<title>` and history automatically, and everything still works as plain navigation without JS.

- **Opt-outs** — add `hx-boost="false"` to forms whose response must re-render the `<html>` tag (lang/theme attributes) or change identity. htmx boost swaps only `<body>`, so these need full loads:
  - the logout form in layout.html (line 34, `<form method="post" action="/logout">`)
  - the login form in `login.html`
  - the register form in `register.html`
  - the profile settings form in `profile.html` (theme/language changes)

  Find each with grep `<form` in those files and add the attribute to the `<form>` tag.

- The inline body scripts (SW registration, favicon tint) are idempotent and safe to re-run on swaps; the favicon `fetch()` is now a cache hit. No change needed.

### B3. Click feedback — `internal/web/assets/app.css`

Append (htmx puts class `htmx-request` on `<body>` during boosted requests, per `hx-indicator="body"`):

```css
/* htmx boost: thin top progress bar while a request is in flight */
body.htmx-request::before {
  content: ""; position: fixed; top: 0; left: 0; right: 0; height: 3px;
  background: var(--accent); z-index: 1000;
  animation: pageload 1s ease-out infinite;
}
@keyframes pageload {
  from { transform: scaleX(0); transform-origin: left; }
  to   { transform: scaleX(1); transform-origin: left; }
}
```

(Use the accent variable name that app.css actually defines — grep `--accent` and match it.)

### B4. Docs

- README.md line ~12: “htmx-ready” → now actually enabled with `hx-boost`.
- README.md line ~23: delete/replace the “placeholder” note; state the vendored htmx version and that boost swaps the body while remaining functional without JS.

**Checkpoint:** ask the user to run `./scripts/dev.ps1 all`, then to click around the running app and confirm navigations feel instant.

---

## Phase C — Cut per-page DB work

### C1. `/friends` N+1 → 2 queries — `internal/httpapp/handlers_pages.go` (`handleFriends`, line 73)

Replace the per-friend `FriendBalance` loop with one `CumulatedBalances` call (method already exists, internal/store/balances.go:33 — same data, same ordering):

```go
friends, err := s.Store.ListFriends(ctx, u.ID)
if err != nil { /* unchanged 500 */ }
cum, err := s.Store.CumulatedBalances(ctx, u.ID)
if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}
balsByFriend := map[int64][]store.CumulatedBalance{}
for _, c := range cum {
	balsByFriend[c.FriendID] = append(balsByFriend[c.FriendID], c)
}
// hidden map unchanged
rows := make([]friendRow, 0, len(friends))
for _, f := range friends {
	rows = append(rows, friendRow{User: f, Balances: balsByFriend[f.ID], Hidden: hidden[f.ID]})
}
```

Keep `Store.FriendBalance` — still used by `handleFriendDetail`.

### C2. `/groups` 2N queries → 2 queries

Add to `internal/store/balances.go` (follow the file's existing `rebind` + scan style):

```go
// GroupMemberCounts returns the member count of every group the user belongs to.
func (s *Store) GroupMemberCounts(ctx context.Context, userID int64) (map[int64]int, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT group_id, COUNT(*) FROM group_users
		 WHERE group_id IN (SELECT group_id FROM group_users WHERE user_id = ?)
		 GROUP BY group_id`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var gid int64
		var n int
		if err := rows.Scan(&gid, &n); err != nil {
			return nil, err
		}
		out[gid] = n
	}
	return out, rows.Err()
}

// UserGroupNets returns the user's net position per group per currency in one
// pass over balance_view (groups list page).
func (s *Store) UserGroupNets(ctx context.Context, userID int64) (map[int64]map[string]int64, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT group_id, currency, SUM(amount) FROM balance_view
		 WHERE user_id = ? AND group_id IS NOT NULL
		 GROUP BY group_id, currency`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]int64{}
	for rows.Next() {
		var gid int64
		var cur string
		var amt int64
		if err := rows.Scan(&gid, &cur, &amt); err != nil {
			return nil, err
		}
		if out[gid] == nil {
			out[gid] = map[string]int64{}
		}
		out[gid][cur] += amt
	}
	return out, rows.Err()
}
```

Rewrite the loop in `handleGroups` (handlers_pages.go:166):

```go
counts, err := s.Store.GroupMemberCounts(ctx, u.ID)
if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}
nets, err := s.Store.UserGroupNets(ctx, u.ID)
if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}
rows := make([]groupRow, 0, len(groups))
for _, g := range groups {
	rows = append(rows, groupRow{Group: g, MemberCount: counts[g.ID], Balances: sortedNets(nets[g.ID])})
}
```

Keep `GroupMembers` and `UserGroupBalances` — still used by `handleGroupDetail`.

### C3. Feed limit + "Show all"

**Store — `internal/store/expenses.go`:**
- Add to `ExpenseFilter` (line 40): `Limit int // max rows returned; 0 = unlimited`.
- Change `queryExpenses` (line 419) to take a limit:

```go
func (s *Store) queryExpenses(ctx context.Context, conds []string, args []any, limit int) ([]*Expense, error) {
	q := `SELECT ` + expenseCols + ` FROM expenses e WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY e.expense_date DESC, e.created_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	// ... rest unchanged
```

- Update ALL callers (grep `queryExpenses(`): `ListFriendExpenses`, `ListGroupExpenses`, `ListActivity` pass `f.Limit`; `ListDirectCollapsible`, `ListGroupCollapsible` (and any other caller) pass `0` — collapse logic must see full history.

**Handlers — `internal/httpapp/handlers_pages.go` + `handlers_helpers.go`:**
- Add `const feedLimit = 100` in handlers_helpers.go.
- In `handleActivity`, `handleFriendDetail`, `handleGroupDetail`, right after `parseFilter(r)`:

```go
showAll := r.URL.Query().Get("all") == "1"
if !showAll {
	filter.Limit = feedLimit + 1 // fetch one extra to detect truncation
}
```

- After the expense query succeeds:

```go
showAllHref := ""
if !showAll && len(expenses) > feedLimit {
	expenses = expenses[:feedLimit]
	q := r.URL.Query()
	q.Set("all", "1")
	showAllHref = r.URL.Path + "?" + q.Encode()
}
```

- Add `"ShowAllHref": showAllHref` to each handler's render data map.

**Templates:**
- `internal/web/templates/partials.html`, inside `{{define "expense_table"}}`: after the closing `{{end}}` of the outer `{{range .Groups}}` (line 89), before `{{else}}`, add:

```html
{{if .ShowAllHref}}<p class="feed-more"><a class="a-btn ghost" href="{{.ShowAllHref}}">{{$root.T "feed.show_all"}}</a></p>{{end}}
```

- Pass it through in the three pages (each has a `{{template "expense_table" (dict ...)}}` call):
  - `activity.html:4` → add `"ShowAllHref" .Data.ShowAllHref`
  - `friend.html` and `group.html` → same addition on their `expense_table` dict.

**i18n — add key `feed.show_all` to all 9 catalogs in `internal/i18n/locales/`:**
en `Show all` · de `Alle anzeigen` · es `Mostrar todo` · fr `Tout afficher` · it `Mostra tutto` · ja `すべて表示` · ko `모두 보기` · zh-Hans `显示全部` · zh-Hant `顯示全部`. Match each file's existing JSON key ordering/style.

Known minor behavior (accepted): clicking a filter chip drops `all=1` (chips rebuild the query from the frozen filter-key list) — the feed just returns to the limited view.

### C4. Tests for Phase C

- `internal/store/filter_test.go` — add `TestExpenseFilterLimit` using the file's existing setup helpers: insert more than N expenses, query with `Limit: N`, assert `len == N` and newest-first ordering.
- New store tests for `GroupMemberCounts` and `UserGroupNets` (same file or a sibling, mirroring `filter_test.go` fixtures): counts match members added; nets match what per-group `UserGroupBalances` returns for the same data.
- `internal/httpapp/feed_test.go` — add a test that creates `feedLimit+1` expenses via the harness, asserts the page body contains the `feed.show_all` link, and that `?all=1` renders all rows (no link).
- Existing `lists_test.go` friends/groups tests must still pass unchanged (they validate the rewritten handlers).

---

## Out of scope (deliberate — do not do)

- No materialized balances ("balances are derived, never stored" is an architecture principle; C1/C2 reduce evaluations instead).
- No SQLite `MaxOpenConns` change (single-writer safety; Phase A removes most per-request DB traffic).
- No TLS/HTTP2/reverse-proxy work.

---

## Final verification (ask the user to run)

1. `./scripts/dev.ps1 all` (or `./scripts/dev.sh all`) — report build/vet/test/coverage output.
2. `python -m graphify update .` — refresh the knowledge graph.
3. Manual, with the app running (`go run ./cmd/gosplit` or `docker compose up --build`):
   - DevTools → Network: first load fetches assets; **subsequent navigations show zero `/static/*` requests** (HTTP cache / SW cache-first).
   - Responses show `Content-Encoding: gzip`.
   - Clicks navigate via htmx (request rows carry `HX-Request: true`) and feel immediate; the top progress bar appears on slower responses.
   - Server JSON logs: compare `dur_ms` for `/friends` and `/groups` before/after (requestLogger already emits it).
   - An account with >100 expenses shows "Show all" on activity/friend/group feeds; `?all=1` shows everything.

---

## Ledger (executor: fill in per phase in docs/perf-plan.md)

- [x] Phase A — shipped: fingerprinted `{{asset}}` template func + immutable/short cache headers in `internal/web/view.go`; all 57 `/static/` refs across 14 templates rewritten; router split into public (no-auth, compressed) static/health routes vs. an authenticated `r.Group` in `internal/httpapp/server.go` (adds `middleware.Compress(5)`, `cacheControl` helper); `sw.js` bumped to `gosplit-v3` with cache-first `/static/*` handling. Tests: `TestAssetURLFingerprinted` (web), `TestStaticCacheHeaders` (httpapp).
- [x] Phase B — shipped: vendored real htmx (htmx version: 2.0.10) replacing the placeholder stub; `hx-boost="true" hx-indicator="body"` on `<body>` in layout.html; `hx-boost="false"` opt-outs on logout/login/magic-link/register/profile forms (they touch `<html lang/data-accent>` or identity); top progress-bar CSS keyed off `.htmx-request`.
- [x] Phase C — shipped: `/friends` N+1 collapsed to one `CumulatedBalances` call + in-memory bucketing; `/groups` 2N queries collapsed to two new store methods `GroupMemberCounts`/`UserGroupNets` (`internal/store/balances.go`); `ExpenseFilter.Limit` + `LIMIT` clause added to `queryExpenses` (collapse-history callers pass `0` = unlimited); `feedLimit=100` with `applyFeedLimit`/`showAllHref` helpers wired into activity/friend/group handlers and the shared `expense_table` partial; `feed.show_all` key added to all 9 locale catalogs. Tests: `TestExpenseFilterLimit`, `TestGroupMemberCountsAndNets` (store), `TestActivityFeedShowAll` (httpapp).
- [x] Docs updated: README (htmx hx-boost note, vendored version), this ledger. Graphify update still pending (ask user to run `python -m graphify update .`).
