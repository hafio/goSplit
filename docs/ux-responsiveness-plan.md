# GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)

Status: approved 2026-09-01. Executor: Opus (medium). Update the Ledger as steps land.

## Ledger

| Step | Item | Status | Notes |
|------|------|--------|-------|
| 1 | hx-disabled-elt double-submit guard | done | 15 forms across 11 templates. Admin's per-user editor form is deliberately excluded: it has 4 submit buttons via `formaction`, and `find` disables only the first, so the wrong button would grey out. |
| 2 | Fix /profile/export boosted anchor | done | |
| 3 | hx-boost=false on forgot/reset forms | done | |
| 4 | Delete embedded assets/nppBackup | done | Verified `icon.svg` intact first; the `.bak` was a stale 2026-07-13 copy. |
| 5 | View Transitions config meta | done | + `prefers-reduced-motion` opt-out in app.css. |
| 6 | Idiomorph 0.7.4 body morphing | done | Tarball verified against the npm sha512 before extracting; file sha256 recorded in the vendored header. Uses `morph:innerHTML` (boost swaps body's children, not the body element). |
| 7 | Refetch-on-focus | done | Guarded by a dirty-form check and a `window.__gsRefreshWired` flag. |
| 8 | Web Push tab nudge | done | Opt-out (`refresh: false`) rather than opt-in, so no Go payload change was needed. |
| 9 | RenderFragment + view.go comment fix | done | Shared `begin()` keeps fragment and full-page headers identical. |
| 10 | Fragment GET branch | done | activity / friend / group feeds. **Balances skipped**: it has no filter bar, so a fragment there would be an unexercised code path. |
| 11 | Single-round-trip simple mutations | **skipped** | See "Deviations" below. |
| 12 | Flash via HX-Trigger header | done, **re-designed** | Implemented as a one-shot cookie. See "Deviations". |
| 13 | Client-side validation | done, **re-designed** | Implemented as an HTML `pattern`. See "Deviations". |
| 14 | Per-action indicators | done | `.frag.htmx-request` dims the region; the page-wide top bar no longer fires for feed refreshes. |
| -- | Tests | done | `internal/web/render_fragment_test.go`, `internal/httpapp/fragment_test.go`, `internal/httpapp/progressive_test.go`; `getHX` helper added to the harness. |
| -- | README + docs | done | New "Client behaviour" section in README; stale comments fixed in `view.go` and `admin_test.go`. |

## Deviations from the approved plan

Three items changed shape once checked against the actual code. Each was
verified in the vendored htmx/CSS source before deciding.

1. **Step 12 — cookie, not `HX-Trigger`.** `HX-Trigger` cannot work here: every
   flash rides a 303, XHR follows redirects transparently, and only the *final*
   response's headers are visible to htmx, so the header would be discarded
   unread. A one-shot `gs_flash` cookie achieves the plan's stated intent (shown
   once, absent from the URL, translated exactly once) and additionally works
   with JavaScript disabled. Value is base64'd because display strings contain
   spaces and quotes, which are not legal raw in a cookie.
2. **Step 13 — HTML `pattern`, not JS validation.** The intended
   "at least one participant" check would have to call `setCustomValidity` on the
   include checkboxes, which are `position:absolute; opacity:0;
   pointer-events:none` (app.css). `reportValidity` on an unfocusable control
   fails to display and can throw — silently blocking *every* submit. Blocking
   submission from JS is also hazardous next to step 1: htmx disables the submit
   button before the request, and cancelling at `htmx:beforeRequest` risks
   leaving it permanently disabled. An HTML `pattern` on the visible amount input
   is honoured by htmx (it calls `checkValidity`/`reportValidity` before a boosted
   submit), needs no new translation strings, and cannot wedge the form.
3. **Step 11 — skipped.** Every candidate mutation (group archive/simplify/
   invite, friend hide) changes regions *outside* the feed fragment: the header
   tag, the member avatars, the settlement list, the net-position hero. A
   single-region swap would leave visibly stale data — worse than the current
   full re-render. Covering them properly needs a whole-`content` fragment, which
   in turn requires moving the flash out of the layout: a materially larger
   change than the plan scoped, for one saved round trip on infrequent actions.
   The frequent interactions (filter, search, paging) *do* get the single-round-
   trip treatment via step 10. Revisit alongside any future `content`-level
   fragment work.

## Context

The frontend was audited against the belief that it was "SSE-driven with component-level
updates". It is not: htmx 2.0.10 with `hx-boost="true"` on `<body>` (layout.html:17) makes every
one of ~75 links/forms an AJAX request, but the server always renders the FULL page
(`Renderer.Render`, internal/web/view.go:217-242, `Cache-Control: no-store`) and htmx swaps the
FULL `<body>`. All mutations are POST-redirect-GET (`redirectFlash`, ~11 call sites). Zero SSE,
zero partial swaps.

An options comparison (SSE, WebSockets, polling, refetch-on-focus, existing Web Push channel,
non-realtime UX levers; all claims verified against source) concluded: **SSE is deferred** -- its
gap (a foregrounded tab going stale during simultaneous edits) is rare for this app and carries
the largest deployment tax (60s `WriteTimeout` at cmd/gosplit/main.go:113, global `Compress(5)`
at server.go:45, reverse-proxy buffering). Scope chosen: **Tier 1 + Tier 2**. WebSockets,
preload/prefetch (dead under `no-store`), stale-while-revalidate pages, and Background Sync were
rejected outright.

Verified bugs this plan fixes along the way:

- `/profile/export` (profile.html:57) is boosted, so `Content-Disposition: attachment`
  (handleExport, handlers_pages.go:530-542) never reaches the browser -- download silently broken.
- No double-submit protection anywhere; fast double-tap fires duplicate POSTs.
- `?flash=` messages are baked into pushed URLs with no `history.replaceState` (zero hits) --
  refresh/back redisplays stale flashes; some call sites double-translate via `T()`.
- Every boosted nav discards scroll/focus/open-`<details>` state (full body innerHTML swap).
- forgot.html/reset.html forms lack the `hx-boost="false"` their siblings (login/register) have.
- `internal/web/assets/nppBackup/*.bak` is swept into `//go:embed assets/*` and web-servable.

Dependency pins (verified via npm registry 2026-08-31):

- **idiomorph 0.7.4** (`dist/idiomorph-ext.min.js`, htmx `hx-ext="morph"`), tarball integrity
  `sha512-uCdSpLo3uMfqOmrwXTpR1k/sq4sSmKC7l4o/LdJOEU+MMMq+wkevRqOQYn3lP7vfz9Mv+USBEqPvi0XhdL9ENw==`.
  Record SHA256 of the vendored file in its header comment.

## Tier 1 -- quick wins (independent, ship in any order)

1. **Double-submit guard**: add `hx-disabled-elt="this"` to submit buttons on all mutating forms --
   expense_form.html (:135), group.html (archive/simplify/invite/settle forms), friend.html
   (hide/delete/settle), collapse.html, settle_group.html, recurring.html, admin.html,
   friends.html, groups.html, import.html, convert.html. Add `:disabled` button styling in
   app.css if not present.
2. **Fix `/profile/export`**: `hx-boost="false"` on the anchor (profile.html:57).
3. **Boost opt-out consistency**: `hx-boost="false"` on forgot.html and reset.html forms.
4. **Delete `internal/web/assets/nppBackup/`** (embedded editor backups; repo-root ./nppBackup is
   not embedded -- leave it).
5. **View Transitions**: `<meta name="htmx-config" content='{"globalViewTransitions":true}'>` in
   layout.html head. Optional CSS to disable under `@media (prefers-reduced-motion)`.
6. **Idiomorph body morphing**: vendor `internal/web/assets/idiomorph-ext.min.js` at 0.7.4 (pin
   above); layout.html: script tag after htmx.min.js, and on `<body>` add `hx-ext="morph"
   hx-swap="morph"` alongside the existing hx-boost. Preserves scroll/focus/open-`<details>`/typed
   form state across every boosted nav. Watch: expense_form.js mutates picker summary text --
   verify morph does not fight it (add morph-ignore/preserve attributes only if a real conflict
   shows in manual testing).
7. **Refetch-on-focus**: small inline script in layout.html -- on `visibilitychange` to visible
   (and `pageshow` with `persisted`), if the page has been hidden > 60s and user is authenticated,
   re-GET `location.href` via `htmx.ajax('GET', location.href, {target:'body', swap:'morph'})`.
   Skip while a form has unsaved input (`document.activeElement` inside a form or any dirty field).
8. **Web Push tab nudge**: extend the push payload (internal/push/push.go / notify_service.go)
   with a `refresh` field; in sw.js's existing `push` listener, after `showNotification`,
   `clients.matchAll({type:'window'})` and `postMessage({type:'refresh'})`; page listener in
   layout.html reuses the step-7 refetch helper. Actor-excluded fan-out already exists
   (notify_service.go:24) -- no recipient changes. Only fires for users who granted push permission.

## Tier 2 -- structural (phased, after Tier 1)

9. **Fragment rendering**: add `RenderFragment(w, status, page, block, vd)` to
   internal/web/view.go (`ExecuteTemplate(w, block, vd)` instead of "layout.html", same no-store
   headers). Feasible as-is: `NewRenderer` (view.go:171) already compiles layout+partials+page
   into one template set, so any `{{define}}` block is addressable. Fix the stale package doc
   comment (view.go:3 -- "Alpine + fragment swaps" was never true).
10. **Fragment GET branch**: `isFragmentRequest(r, id)` helper comparing the `HX-Target` request
    header to a fragment DOM id; branch GET handlers for /groups/{id}, /friends/{id}, /activity,
    /balances. Shared `{{define "frag_feed"}}` in partials.html wrapping the filterbar +
    expense_table pair (byte-identical today across group.html:81-82, friend.html:32-33,
    activity.html:3-4); `frag_balances` / `frag_group_balances` blocks for the stat regions.
    Wrapper divs must exist in EVERY page state (empty feed, archived) or refreshes silently no-op.
11. **Single-round-trip simple mutations**: for same-page toggles (group archive/simplify, friend
    hide/unhide, invite, recurring delete), when the request is boosted-with-HX-Target, return the
    updated fragment (HTTP 200) directly from the POST handler instead of 303+full GET; templates
    gain `hx-target="#group-feed" hx-swap="outerHTML"` (or morph) on those forms. Keep PRG for
    cross-page mutations (expense create/delete, settle flows, auth) and for non-htmx requests --
    the PRG path stays as the no-JS fallback everywhere.
12. **Flash via HX-Trigger header**: replace `redirectFlash`'s `?flash=`
    (handlers_pages.go:679-684, ~11 call sites + /login?flash literal at :527) with an
    `HX-Trigger: {"flash": {...}}` response header on htmx requests; small listener in layout.html
    renders into the existing `.flash` div (auto-dismiss). Non-boosted flows (login/register/
    profile, no-JS) keep the query param -- but translate exactly once server-side (fix the
    double-`T()` path where handlers pass pre-formatted strings, e.g. handlers_archive.go:65/110).
13. **Client-side validation**: extend expense_form.js to pre-check amount format and >=1
    participant before submit, reusing its existing live-preview event wiring; inline error text,
    server remains source of truth.
14. **Per-action indicators**: move `hx-indicator` off `body` for fragment actions to per-form
    spinners; disabled+spinner state on the acting button (composes with step 1). No true
    optimistic writes -- financial data waits for the server response.

## Explicitly deferred / rejected

- SSE (internal/events broker design): revisit only if live/collaborative becomes a real
  requirement; then prefer SSE over WebSockets, built on step 9-10's fragment layer. The full SSE
  design (broker, /events handler, htmx-ext-sse 2.2.4 pin, WriteTimeout/Compress/sw.js handling)
  was drafted and is recoverable from the planning session if needed.
- Preload/prefetch (any flavor): no-op under `Cache-Control: no-store`.
- SW stale-while-revalidate for pages, Background Sync queueing: wrong for financial data /
  unsupported on Safari.

## Tests (same change as the code they cover)

- internal/web/view_test.go: RenderFragment renders only the named block, no-store headers,
  error on unknown block.
- internal/httpapp/feed_test.go (new): `HX-Target: group-feed` GET returns only the fragment div
  for group/friend/activity/balances; without the header returns full layout.
- internal/httpapp handlers tests (extend): simple-mutation POSTs with HX-Target return 200 +
  fragment; without htmx headers still 303 (PRG fallback preserved, redirect semantics unchanged).
- Flash: htmx request gets HX-Trigger header and clean Location; non-htmx keeps ?flash=;
  single-translation asserted for the pre-formatted-string call sites.
- Push payload: nudge field present, recipients still actor-excluded (regression guard).
- Template assertions (lists_test.go pattern): hx-disabled-elt present on mutating forms;
  hx-boost=false on export link + forgot/reset forms; fragment wrapper ids present in empty-state
  renders.

## Docs (same change)

- README.md: architecture/stack notes -- idiomorph morphing swaps, View Transitions, fragment
  rendering via HX-Target, HX-Trigger flash, refetch-on-focus, push tab nudge; one line noting
  SSE was evaluated and deferred (so the decision is not re-litigated).
- view.go package comment fix (step 9).

## Verification (user-run)

- `go build ./...` ; `go vet ./...` ; `go test ./... -race`
- Manual, mobile emulation + desktop: scroll deep into activity, tap an expense, go back --
  scroll preserved (morph); double-tap a submit -- one POST in Network tab; /profile/export
  downloads; archive a group from the omenu -- single request, feed region updates in place, no
  page jump; refresh after a flash -- flash does not reappear; background the tab >60s while
  another user adds an expense, refocus -- feed refreshes; offline mode still serves /offline.
- `graphify update .` afterward.

---

# Round 2 -- guaranteed freshness + design cleanup

Executed 2026-09-06. Driven by a new hard requirement: **what a user is looking at must be
<=10s stale while the page is visible**, and any caching must come with a mechanism that
guarantees it. A second review (3 reviewers + judge, every claim re-verified against source)
also surfaced four defects in the Round 1 code.

## Round 2 Ledger

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Flash cookie missing `Secure` | done | `setFlash`/`takeFlash` are now `*Server` methods carrying `s.Auth.Secure`, matching the session/CSRF cookies. It was the only cookie in the app without it. |
| 2 | Background render eats the flash | done | Refresh and poll send `X-Background`; `vdPage` skips `takeFlash` when present, so a background GET can no longer swallow a confirmation meant for a redirect target. |
| 3 | convert.js permanently "dirty" | done | Derived writes go through `setValue`, which moves `defaultValue` too. Chip clicks deliberately do not -- those are real edits. |
| 4 | Page scripts never re-wire | done | All four scripts gained `wire()` + `htmx:load` + a `WeakSet` guard. Worse than the plan assumed: `bank.js`/`push.js` bound on `DOMContentLoaded`, which never fires again after a boosted swap, so their buttons were dead on *any* navigation -- a pre-existing bug. A `WeakSet` rather than a `data-wired` attribute, because a morph would strip the attribute and cause double-binding. |
| 5 | Fragment registry | done | `fragments` map in `view.go`, validated in `NewRenderer` (fail-loud). The three copy-pasted `isFragmentRequest` branches are gone from `handlers_pages.go`; `Server.render` dispatches. |
| 6 | Content fragment for every page | done | `ViewData.Page`/`Path` added; `pollable` template func. |
| 7 | Content-hash 204 polling | done | `RenderFragment` buffers, fingerprints (sha256/8 bytes), always sets `X-Fragment-Version`, answers 204 when `?v=` matches. Layout emits the poller on pollable pages only. |
| 8 | `Server-Timing` | done | `render;dur=<ms>` on HTML responses, omitted on credential pages. |
| -- | Tests | done | Renderer: version/204/stale, fingerprint tracks content, registry validity, `pollable` coverage. httpapp: poller presence per page, end-to-end 204, fingerprint changes when a friend is added, `Server-Timing`, `Secure` flag, `X-Background` leaves the flash. |
| -- | Docs | done | README "Client behaviour" rewritten for the registry, freshness and measurement. |

## Design decisions worth keeping

- **Poll, not SSE.** The requirement is a bar (<=10s), not a latency race. On a single-container,
  SQLite, small-user deployment SSE's ops cost (proxy buffering, the 60s `WriteTimeout` carve-out,
  reconnect storms) buys nothing a poll does not already deliver. The poller and SSE would trigger
  the *same* regions off the *same* registry, so SSE stays a drop-in upgrade rather than a rewrite.
- **Hash the rendered block, not an `updated_at`.** Correct by construction for anything the
  template shows -- a renamed friend, a changed member list -- with no schema to keep in step and
  no new columns. Costs one render per poll, which is microseconds against SQLite. Assumes the
  block has no per-request noise; the double-submit CSRF token is per-session, so forms inside a
  polled page do not defeat it.
- **Pause on interaction, not just on hidden.** The trigger filter also requires no focused
  control and no open `details` inside `#content`, so a poll can never snap a filter panel shut or
  rewrite a search box mid-type. htmx re-schedules after a filtered-out tick, so polling resumes
  by itself.
- **Both render paths now buffer.** `Render` was committing a status before it could fail, so a
  template error emitted a half-written page and was swallowed to `io.Discard`. Buffering fixes
  that, makes errors actionable, and is what allows the fingerprint and timing headers at all.

## Still deferred

- **DB tuning**: `SetMaxOpenConns(1)` is a deliberate single-writer choice; `PRAGMA
  synchronous=NORMAL` is absent; `handleGroupDetail` makes two `balance_view` passes; `nameCache`
  misses are per-row. Revisit only with `Server-Timing` numbers, and re-run the gates.
- **`hx-target="main"` navigation** (persist header/nav, halve nav payloads, subsume Round 1's
  skipped step 11). Its prerequisite -- a universal `content` fragment -- now exists.
- **SSE**, as above.
