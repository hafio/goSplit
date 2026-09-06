# Architecture

How GoSplit is built and why. For contributors and for operators who want to understand
what they are running. Nothing here is needed to use or deploy it.

## Stack

- **Go 1.27**, [chi](https://github.com/go-chi/chi) router, standard-library
  `html/template` views. One statically linked binary, no CGO.
- **SQLite** through the pure-Go `modernc.org/sqlite`, or **PostgreSQL** through pgx. The
  engine is picked from `DATABASE_URL`. Migrations are embedded per engine and applied
  at startup. On SQLite the pool is pinned to one connection (WAL, a 5 s busy timeout and
  foreign keys set in the DSN), so every request serialises on it; that is why an
  in-process restore raises a maintenance gate and why a dump opens its own read-only
  connection.
- **Sessions, CSRF and passwords** are hand-rolled on the standard library: argon2id
  hashes, server-side sessions, secure cookies whose `Secure` flag follows `BASE_URL`.
- **Frontend**: server-rendered HTML with a hand-written stylesheet and a few small vanilla
  scripts, enhanced by htmx 2.0.10 and idiomorph 0.7.4 (both vendored under
  `internal/web/assets/`, the idiomorph file with its upstream hashes recorded in the
  header). No CSS or JavaScript framework. Everything is embedded with `go:embed`.
- **Background work** runs in-process: a leader-locked scheduler and a per-process
  janitor. No queue, no cache server.

The build deliberately avoids code generators (no `templ`, no `sqlc`, no Tailwind CLI) so
that `go build` is the whole build.

## Package map

```text
cmd/gosplit          entrypoint: config, CLI subcommands, startup order, graceful shutdown
internal/
  config             typed env config, engine selection, fail-loud validation
  store              database/sql layer, embedded migrations, balance_view queries,
                     cross-engine "?" to "$n" rebinding, leader locks
  split              the split engine: EQUAL, PERCENTAGE, EXACT, SHARE, ADJUSTMENT
  balance            debt simplification (min-cash-flow)
  money              int64 minor-unit parse and format; money is never a float
  auth               argon2id, sessions, CSRF, RequireUser/RequireAdmin middleware
  service            business logic; handlers stay thin
  httpapp            chi router, HTTP handlers, fragment dispatch, maintenance gate
  web                html/template renderer, fragment registry, embedded templates
                     and assets, themes
  i18n               message catalogues (locales/*.json) and language detection
  mail               SMTP mailer with a log-only fallback
  push               Web Push (VAPID) sender
  currency           exchange-rate providers (Frankfurter, Open Exchange Rates, none)
  bank               bank.Provider interface and the Plaid implementation
  backup             archive format, dump, validate, restore, auto-restore, recovery
  scheduler          recurrence generation, cache cleanup, scheduled backups
```

Handlers parse input and render; services own the rules; the store owns SQL. Nothing in
`internal/` reaches for a global: the configuration and dependencies are passed in.

### Startup order

`config.Load` (fatal on misconfiguration) -> repair any interrupted upload swap -> open
the database and run migrations -> startup auto-restore if configured -> construct
mailer, services, auth, renderer (the fragment registry is validated here) -> start the
janitor -> start the scheduler if enabled -> listen. Auto-restore sits where it does
because the schema is current and nothing can write yet.

## Data model

### Balances are derived, never stored

Expenses and their signed, zero-sum participant rows are the single source of truth. A
SQL view, `balance_view` (one definition per dialect, same shape), canonicalises each
debtor/payer pair, sums signed shares per `(pair, group, currency)`, and unions both
directions. `amount > 0` in row `(user, friend)` means the friend owes the user. Balances
stay per currency; nothing ever converts implicitly.

Archived groups are excluded from the aggregate views by a `NOT IN (archived groups)`
clause, which is how "archive" removes a group from your totals without touching a row.

### Split inputs are kept only for the edit form

The raw values a user typed (percentages, share weights, exact amounts, adjustments) are
stored alongside in `expense_split_inputs`, purely so that editing restores the form as
entered. Nothing reads that table to compute money, so it cannot affect a balance. An
expense with no row there (one saved before the table existed, or a settlement, conversion
or archive entry that never went through the split engine) falls back to reconstructing
the form from the stored amounts.

### Optimistic concurrency on expenses

Every expense update and delete is guarded by a `version` column. The edit form and the
delete button post the version their page was rendered from; a save or delete whose
version no longer matches is refused with `409 Conflict` instead of overwriting someone
else's change. Rows start at version 1, so a request carrying no version token matches
nothing and fails closed.

### Collapsing history

Collapse moves the original rows into `archived_expenses` and
`archived_expense_participants`, which have no read path in the application (they exist
for audit and for backups), and inserts one balance-preserving "Historical Transactions"
expense per currency with a CSV of the originals in its note.

## Request handling

The router applies request-id, real-ip, logging, recovery, gzip compression and the
maintenance gate globally, then splits into public routes (auth pages, `/healthz`, static
assets, the PWA files, the token-gated restore status), signed-in routes, and admin
routes. CSRF is verified on every mutating request. The restore upload declares its own
middleware chain so that a body size limit is installed ahead of its CSRF check, which
would otherwise buffer the whole multipart upload before the limit applied; CSRF still
runs.

The maintenance gate answers `503` with `Retry-After` to every non-safe request while an
in-process restore holds the database.

## Client behaviour

Everything here is progressive enhancement layered on the same server-rendered HTML.
With JavaScript off, every flow falls back to plain navigation and POST-redirect-GET.

### Navigation

`<body>` carries `hx-boost="true"` with `hx-ext="morph" hx-swap="morph:innerHTML"`, so a
link or form submit fetches the next page over XHR and **morphs** the existing DOM into it
rather than replacing it. Open menus, focus and half-typed fields survive; page scripts
re-run because htmx clones parsed `<script>` nodes before the swap. The `htmx-config`
meta enables View Transitions, and `prefers-reduced-motion` disables the animation.

Forms that change identity or download a file opt out with `hx-boost="false"`: login
(both the password and the magic-link form), register, forgot and reset password, logout,
profile, the data export link, and the backup download, upload and confirm forms. A
boosted anchor would swallow a `Content-Disposition: attachment`.

### Fragments

`Renderer.RenderFragment` executes one named template block instead of the whole layout.
A page declares its swappable regions in the `fragments` registry in `internal/web/view.go`
as `page -> HX-Target id -> block`; the registry is validated when the renderer is built,
so a typo fails the process at startup rather than returning a 500 on the first swap.
`Server.render` consults it, so handlers carry no fragment branches: a request whose
`HX-Target` names a registered region gets that block, anything else (including a boosted
navigation, which targets the body) gets the full page. Eight pages register a `content`
region: balances, friends, friend, groups, group, activity, expense_detail and
recurring. The three feeds add `#activity-feed`, `#friend-feed` and `#group-feed` so that
filtering and paging swap only the feed. Form pages are deliberately absent, so nothing
can re-render a form under the user.

### Freshness

On the eight registered pages, the layout emits a poller that re-requests the page's
`content` block every 10 seconds, but only while the tab is visible and the user is not
busy in it: no focused control and no open `<details>` inside `#content`. The server
fingerprints the rendered block (first 8 bytes of a SHA-256), returns it in
`X-Fragment-Version`, and answers `204 No Content` when the echoed `?v=` matches, so a
quiet poll costs a header exchange. Fingerprinting the output rather than an `updated_at`
is correct by construction for anything the template shows, with no schema to keep in
step. The poll follows the browser's current URL, so filters pushed into history are
respected, and it morphs with `ignoreActiveValue` as a second guard for typed input.

A backgrounded tab also re-requests and morphs the page when refocused after a minute, on
back/forward-cache restore, and when a Web Push delivery tells open tabs to refresh. The
refresh is skipped while any form control differs from its default value.

Background renders send `X-Background: 1`, so the server does not let them consume a
one-shot flash meant for a real navigation. The flash itself rides a short-lived
`gs_flash` cookie (set on redirect, deleted on read) rather than a query parameter, so it
shows exactly once and never becomes part of a bookmarkable URL. The flash element sits
outside `#content` so a poll cannot wipe an unread message.

### Safeguards

- Mutating forms carry `hx-disabled-elt="find button[type=submit]"`, so a double-tap
  cannot fire the same POST twice. The admin's per-user editor form is excluded because it
  has several submit buttons; the bank sync form currently lacks the attribute too.
- Amount inputs carry an HTML `pattern` that htmx checks before a boosted submit, so an
  obvious typo is caught with the browser's own localised message. The pattern is a
  superset of what `money.Parse` accepts (it does not know the currency's decimal places);
  the server remains the source of truth.
- HTML responses are `Cache-Control: no-store`; static assets are content-hashed and served
  with a one-year immutable cache.

### Measurement

HTML responses carry `Server-Timing: render;dur=<ms>` so every page load is a profiling
sample. Credential pages and the generic message page (which also serves `/offline`) are
excluded.

### Progressive web app

`/manifest.webmanifest` and `/sw.js` are served from the origin root outside the
session-required routes. The service worker precaches the shell (`/offline`, the
stylesheet, the icon, the manifest) and serves fingerprinted `/static/*` cache-first.
Navigations and other GETs are network-first with a ten-second cap; successful responses
are copied into the worker's cache and used only when the network fails or times out,
with `/offline` as the fallback for a page never cached. A push payload can carry
`refresh: false` to show a notification without nudging open tabs.

## Decisions worth keeping

- **Poll, not SSE.** The requirement is a freshness bar (at most about 10 seconds stale
  while visible), not a latency race. On a single-container deployment SSE's operational
  cost (proxy buffering, write-timeout carve-outs, reconnect storms) buys nothing a poll
  does not deliver. The poller and a future SSE channel would trigger the same regions off
  the same registry, so SSE stays a drop-in upgrade. Evaluated and deferred; do not
  re-propose without a live-collaboration requirement.
- **Hash the rendered block, not a timestamp.** No new columns, correct for renamed
  friends and changed member lists, costs one render per poll.
- **Pause on interaction, not just on hidden.** A poll must never snap a filter panel shut
  or rewrite a search box mid-type.
- **Both render paths buffer.** A template error produces an actionable 500 rather than a
  half-written page, and buffering is what makes the fingerprint and timing headers
  possible.

Still deferred: a `hx-target="main"` navigation mode that keeps the header across pages,
database tuning (`synchronous=NORMAL`, a second `balance_view` pass on the group page),
and SSE as above. Revisit with `Server-Timing` numbers in hand.

## Background jobs

- **Scheduler** ([`SCHEDULER`](configuration.md#scheduler)): ticks once a minute under a
  `cleanup` leader lock. Each tick generates due recurring expenses exactly once, deletes
  expired sessions and tokens, prunes cached exchange rates older than
  `CACHE_RETENTION_INTERVAL` (`CLEAR_CACHE_CRON_RULE` is read but not yet honoured), and
  fires a scheduled backup when `BACKUP_CRON` is due, in its own goroutine under a
  separately renewed `scheduled-backup` lock. Cron rules are evaluated in UTC.
- **Janitor**: per process, every five minutes, not leader-gated. Expires finished
  backup/restore jobs, discards abandoned restore uploads, and sweeps stale `.partial`
  archives.
- **Auto-restore** runs once at startup under an `auto-restore` lock renewed for as long
  as it takes.

Leader locks are rows in `scheduler_locks` with a TTL; an expired row can be taken over,
so a crashed holder self-heals within the TTL.

## Backup archives

One sealed file, `.gsbak`. A plaintext JSON header (format version, creation time, source
engine, GoSplit version, KDF and AEAD parameters, a non-secret key fingerprint, the
migration list) is followed by the payload: a gzipped tar, sealed in 1 MiB AES-256-GCM
chunks whose nonces carry a counter and a last-chunk flag, with the header bytes as
associated data. Truncation, reordering or splicing is an authentication failure, not
partial data.

The key is derived from `SESSION_SECRET` with scrypt, then split with HKDF into a sealing
key and a fingerprint key. The fingerprint is checked before any decryption so a wrong
secret fails with a clear message.

Inside the tar: `manifest.json` (row count per table, upload count and bytes, the sorted
migration list), one or more `tables/<name>/<nnnn>.jsonl` parts per registered table
(parts are bounded at 8 MiB and numbered from `0000`; an empty table still gets part
`0000`, so a missing table is detectable), and `uploads/<path>` for the upload tree. Rows
are positional JSON arrays in the column order fixed in
`internal/backup/schema.go`; column names never appear in archive content, and every
identifier in generated SQL comes from that registry, never from the archive. Integers are
quoted strings so money survives `encoding/json`; booleans are JSON booleans bound
per-engine; JSON-text columns are copied byte for byte.

Validation builds one index of the tar and reuses it for the restore: every registered
table must be present as a gap-free, duplicate-free run of parts from `0000`, unknown
entries and duplicate names fail, upload paths that escape the upload directory fail, and
every size limit is measured from bytes read, not from header claims. Restore runs the deletes and inserts in one transaction (with
`LOCK TABLE ... ACCESS EXCLUSIVE` on PostgreSQL), resets PostgreSQL sequences, verifies
row counts, commits, then swaps the upload directory with two renames.

## Internationalisation

Catalogues are `internal/i18n/locales/<lang>.json`, embedded and loaded by listing the
directory, so a new locale is one new file. The language is the user's profile setting,
else the best `Accept-Language` match, else English. Templates call `.T "key"`.

## Versioning

The git tag is the single source of version truth. It reaches the binary through
`-ldflags "-X main.version=..."`, and from there the startup log, the page footer, the
`/healthz` body and every backup manifest. An unstamped build reports `dev`.

## See also

- [development.md](development.md) -- building, testing and extending
- [configuration.md](configuration.md) -- the settings named above
- [backup-restore.md](backup-restore.md) -- the operator's view of archives
- [features.md](features.md) -- what all of this delivers
