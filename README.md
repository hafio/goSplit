# GoSplit (Go rebuild)

A self-hostable, open-source expense-splitting app — a Splitwise alternative —
rebuilt as a **single static Go binary** with a server-rendered, thin-client UI.
Recreates GoSplit's feature set with **full parity except OAuth/OIDC** (auth is
magic-link **+ password**), plus expense/activity **filtering** and **moving
expenses between groups**.

## Stack

- **Go 1.26+**, [chi](https://github.com/go-chi/chi) router, **stdlib
  `html/template`** views, progressive-enhancement HTML with **htmx `hx-boost`**
  (AJAX page swaps for snappy navigation; every flow still works without JS).
- **SQLite (default, pure-Go `modernc.org/sqlite`)** or **PostgreSQL** — chosen
  by the `DATABASE_URL` scheme. No CGO; one binary.
- Hand-rolled sessions (`argon2id` passwords, CSRF, secure cookies), embedded
  migrations, `go:embed` templates + assets.

> **Note on this build.** Per the self-contained-build goal, views use stdlib
> `html/template` (not `templ`) and queries use `database/sql` (not `sqlc`) —
> both are documented alternatives in the spec. Styling is a hand-written
> compact CSS in place of the Tailwind CLI (swap in `web/assets/app.css`).
> `web/assets/htmx.min.js` vendors htmx 2.0.10 with `hx-boost` enabled on
> `<body>` — navigation swaps the page body over AJAX instead of a full reload.
> Forms that change identity or theme (login, register, logout, profile) and the
> data export link opt out with `hx-boost="false"`. All core flows still work
> without JavaScript.

### Client behaviour

Everything below is progressive enhancement layered on the same server-rendered
HTML; with JavaScript off, every flow falls back to plain navigation and
POST-redirect-GET.

- **Morphing swaps.** `web/assets/idiomorph-ext.min.js` vendors idiomorph 0.7.4
  (pinned; the file header records the npm tarball and file hashes). `<body>`
  carries `hx-ext="morph" hx-swap="morph:innerHTML"`, so a boosted navigation
  morphs the existing DOM instead of replacing it — open menus, focus and
  half-typed fields survive. Page scripts still re-run: htmx rewrites parsed
  `<script>` nodes into executable clones before the swap.
- **View transitions.** The `htmx-config` meta enables `globalViewTransitions`;
  `prefers-reduced-motion` disables the animation in `app.css`.
- **Fragment rendering.** `Renderer.RenderFragment` executes one named block of a
  page instead of the layout. A request whose `HX-Target` names a region (see
  `isFragmentRequest`) gets just that region, so filtering or paging a feed
  re-renders the feed, not the whole page. The feed wrappers are
  `#activity-feed`, `#friend-feed` and `#group-feed`; an expense row inside them
  opts back out to a full page swap. Anything else — including a boosted
  navigation, which targets the body — takes the full-page path.
- **Double-submit guard.** Mutating forms carry
  `hx-disabled-elt="find button[type=submit]"`, so a fast double-tap cannot fire
  the same POST twice.
- **Client-side amount check.** The amount inputs carry a `pattern` matching what
  `money.Parse` accepts. htmx runs HTML validation before a boosted submit, so a
  typo is caught with the browser's own localized message instead of a round
  trip. The server remains the source of truth.
- **One-shot flash.** Confirmations ride a short-lived `gs_flash` cookie
  (`setFlash`/`takeFlash`) rather than a `?flash=` query param, so the message is
  shown exactly once and never becomes part of a bookmarkable URL.
- **Staying current.** A backgrounded tab re-requests and morphs the current page
  when it is refocused after a minute, on bfcache restore, and when a Web Push
  delivery nudges open tabs (`sw.js` posts a `refresh` message). The refresh is
  skipped while a form holds unsaved input. Live server push (SSE) was evaluated
  and deliberately deferred — see `docs/ux-responsiveness-plan.md`.

## Quick start

### Docker (SQLite, no DB container)

```sh
export SESSION_SECRET=$(openssl rand -base64 32)
docker compose up --build
# open http://localhost:8080
```

### Local

```sh
cp .env.example .env      # set SESSION_SECRET (required)
export $(grep -v '^#' .env | xargs)   # or use direnv
go run ./cmd/gosplit
```

Migrations run automatically on boot. Without SMTP configured, magic-link and
password-reset links are printed to the logs.

### PostgreSQL

Set `DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=disable` and
uncomment the `db` service in `docker-compose.yml`.

## Architecture

```
cmd/gosplit          entrypoint (config, graceful shutdown, scheduler)
internal/
  config              typed env config + engine selection
  store               database/sql layer, embedded per-engine migrations,
                      BalanceView queries, cross-engine `?`→`$n` rebinding
  split               the split engine (EQUAL/PERCENTAGE/EXACT/SHARE/ADJUSTMENT)
  balance             debt simplification (min-cash-flow)
  money               int64 minor-unit parse/format (money is never a float)
  auth                argon2id, sessions, CSRF, middleware
  service             business logic (handlers stay thin)
  httpapp             chi router + HTTP handlers
  web                 html/template renderer + embedded templates/assets
  mail                SMTP mailer (log-only fallback)
  scheduler           periodic cleanup with a DB leader lock
```

### Balances are derived, never stored

Expenses + their signed, zero-sum `ExpenseParticipant` rows are the single
source of truth. Balances come from a SQL **`balance_view`** (per-dialect, but
portable): it canonicalizes each debtor↔payer pair, sums signed shares per
`(pair, group, currency)`, and `UNION ALL`s both directions. `amount > 0` in
row `(user, friend)` means **friend owes user**. Balances stay **per currency**.

The raw per-participant split inputs a user typed (percentages, share weights,
exact amounts, adjustments) are kept alongside, in `expense_split_inputs`, purely
so the edit form can restore them. Nothing reads that table to compute money, so
it cannot affect a balance; an expense with no row there -- one saved before the
table existed, or a settlement/conversion/archive that never runs through the
split engine -- falls back to reconstructing the form from the stored amounts.
Every expense update *and* delete is guarded by a `version` column: the edit form
and the delete button both post the version their page was rendered from, and a
save or delete that no longer matches the stored row is refused with a conflict
instead of silently overwriting -- or discarding -- someone else's edit. Rows
start at version 1, so a request carrying no version token matches nothing and
fails closed.

## Features

Auth (magic-link + password, register/login/change/forgot/reset, admin
auto-promotion, deactivation, plus an admin console to create users, edit any user's
details, set/reset a password (revoking their sessions), and mint a sign-in link), friends (add/hide/delete, per-friend balances +
filtered history that includes shared-group expenses), groups (create/join-link, add a
member by picking an existing friend by name or inviting anyone else by email,
**archive** — hides the group from the normal lists, aggregate balances and activity feed
into a collapsed Archived section that still shows its own unsettled debt, with its
expenses viewable via Activity's off-by-default Archived filter; a debt-simplification
toggle that switches between the minimal transfer set and raw pairwise balances,
detailed balances, **move expenses** between/into/out of groups via a guided
re-split; editing restores the original method and the values as entered rather
than reconstructing them from the amounts, and a settlement edits as a settlement
-- amount, date, note, group -- with its pair, direction and currency fixed;
re-denominating one would leave the debt it cleared outstanding, so that means
deleting it and recording it again), expenses (all
five split methods + **scoped settlements** -- both ways (settle what
you owe or record what you're owed, direction derived server-side from the balance),
per pair or whole-group (one action records the minimum set of transfers that nets
every member), reversed by deleting the settlement; settling from a group clears that
group's balance, settling from a friend clears the cross-group net (see
[docs/settlements.md](docs/settlements.md)); editing or deleting an expense is limited
to its own members (payer/creator/participant) — soft-delete, receipts,
negative amounts; the add form shows and lets you switch the target — a group or
direct; a free-text note per expense; and **archive history** — collapse all
transactions before a date into one balance-preserving "Historical Transactions"
entry per currency, moving the originals into an archive table and keeping a CSV
audit in the note), live per-currency balances & a filtered activity feed, an
installable PWA (manifest + offline service worker).

**Filtering (§5.2):** description (`*` wildcard), amount range, date range,
group scope, and an Activity-only **archived** toggle (off by default; surfaces
expenses from archived groups) — combine with AND, travel as GET query params,
and never affect computed balances.

**Currency conversion:** pluggable rate providers (Frankfurter / OpenExchangeRates,
selected by `CURRENCY_RATE_PROVIDER`) with a DB rate cache; conversions create a
linked expense pair (`/friends/{id}/convert`). The page shows and lets you adjust
the exchange rate, pick the direction (who owes whom, prefilled from the current
balance), and keeps the rate and both amounts in sync live; the from- and to-amounts
you confirm are stored verbatim (no float drift).

**Recurring expenses:** schedule generation from a template expense with a
standard 5-field cron rule (`/recurring`); the in-process scheduler generates due
expenses exactly once under a DB leader lock (gated by `SCHEDULER`).

**Notifications:** Web Push via VAPID (set `WEB_PUSH_PUBLIC_KEY`/`_PRIVATE_KEY`/
`_EMAIL`; enable per-device on the profile page) plus email on new expenses.
Push and email are optional — unset keys/SMTP disable them cleanly.

**Localization:** message catalogs in `internal/i18n/locales/*.json`; language is
picked from the user's preference then `Accept-Language`. Adding a locale is just
a new JSON file — no code change (set your language on the profile page, e.g. `es`).

**Appearance:** a Splitwise-style UI — icon buttons, initials avatars, month-grouped
expense feed with a per-row "you lent / you borrowed" column, tappable pickers on the
add-expense form, and designed empty states. The accent theme is user-selectable from a
named palette (defaulting to a gentle burgundy) on the profile page and stored per user;
every theme ships light and dark variants, and the dollar-sign favicon recolors to match.
No CSS framework or JS framework — hand-written CSS plus one small vanilla script, all
embedded in the binary.

**Optional modules (feature-flagged, disable cleanly when unset):**
- *Splitwise import* (`/import`) — imports friends + groups (not expenses, per the
  reference) using a user-supplied Splitwise API key; idempotent per group.
- *Bank sync* (`/bank`) — [Plaid](https://plaid.com) behind a pluggable `bank.Provider`
  interface (enabled when `PLAID_CLIENT_ID`/`PLAID_SECRET` are set): connect via
  Plaid Link, sync transactions, and convert one into a prefilled expense.

## Configuration

See [.env.example](.env.example) for the full, authoritative env table.
`SESSION_SECRET` is required.

## Testing

```sh
go test ./...                       # unit + integration (SQLite)
go test -cover ./internal/...       # coverage
```

100% coverage is targeted on the split engine, balance view, debt
simplification, and money math; the Golden Scenario (`internal/store`) is a
committed regression fixture. To run the Postgres data-layer tests, set
`TEST_POSTGRES_URL`.

## Development & CI

Mirrored task runners live in [`scripts/`](scripts/) -- `dev.sh` (bash) and
`dev.ps1` (PowerShell), behaviorally identical. They are the only place that
knows how to build, test, lint or scan this repo; CI calls task names only.
Each task truncates `scripts/logs/<task>.log`, tees combined output to it, and
closes with a footer line (`<ISO-8601> | <task> | <secs>s | OK|FAILED`).

```sh
./scripts/dev.sh build   # -> dist/gosplit-<os>-<arch>, version stamped in
./scripts/dev.sh vet
./scripts/dev.sh test
./scripts/dev.sh cov     # profile -> logs/coverage.html, prints the total
./scripts/dev.sh image   # build the distroless image (needs Docker)
./scripts/dev.sh scan    # govulncheck + Trivy on a freshly built image
./scripts/dev.sh up      # docker compose up -d   (down to stop)
./scripts/dev.sh full    # pre-tag sweep: build vet test cov image scan graphify
```

Two aggregates, and only two: `all` = `build vet test` (the fast inner loop,
and what CI runs as `all scan`), `full` = the pre-tag sweep above.

`build`, `vet`, `test` and `scan` are **fatal**. `scan` fails on any CVE
govulncheck reports, and on any fixable CVE Trivy reports; an unfixable one
Trivy finds warns and passes, since there is nothing to act on. It
runs `go tool govulncheck` (a pinned `tool` directive in `go.mod`, never
`go run pkg@version`, which would ignore the toolchain pin) and then Trivy as a
container against a freshly built image -- no local Trivy install needed. The
image half warns and skips where the Docker daemon is absent or not running
linux containers, so the Windows leg checks dependencies only.

`graphify` is local-only and skips when `CI` is set. Overrides: `VERSION`,
`IMAGE_NAME`, `TRIVY_IMAGE`, `TARGET_OS`/`TARGET_ARCH` (cross-compilation).

### Versioning and release

The git tag is the single source of version truth -- no VERSION file, no
hardcoded const. `vMAJOR.MINOR.PATCH` reaches the binary through
`-ldflags "-X main.version=..."`, and the image takes the tag with the `v`
stripped (`ghcr.io/hafio/gosplit:1.0.0`). A dev build reports
`git describe --tags --always --dirty`.

```sh
gosplit --version        # or: gosplit version
```

GitHub Actions:

- **[`.github/workflows/ci.yml`](.github/workflows/ci.yml)** -- `all scan` on
  `ubuntu-24.04` and `windows-2025`, uploading `scripts/logs/`. No automatic
  trigger except PRs that touch the workflows or the dev scripts (which is what
  gates Dependabot's action bumps); otherwise manual or called by `tag.yml`.
- **[`.github/workflows/tag.yml`](.github/workflows/tag.yml)** -- the only
  automatic pipeline. Push a `v*` tag: gates -> multi-arch image to GHCR -> Trivy
  gate on the pushed digest -> GitHub Release, created last. A failure anywhere
  leaves no release.

Nothing runs on an ordinary push, so run `./scripts/dev.sh full` on a clean
checkout before tagging -- it is the only check that catches a file you never
committed.

## License

MIT (matching the upstream project's spirit).
