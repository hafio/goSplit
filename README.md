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
> Forms that change identity or theme (login, register, logout, profile) opt
> out with `hx-boost="false"`. All core flows still work without JavaScript.

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

## Features

Auth (magic-link + password, register/login/change/forgot/reset, admin
auto-promotion, deactivation, plus an admin console to create users, edit any user's
details, set/reset a password (revoking their sessions), and mint a sign-in link), friends (add/hide/delete, per-friend balances +
filtered history that includes shared-group expenses), groups (create/join-link/invite,
**archive** — hides the group from the normal lists, aggregate balances and activity feed
into a collapsed Archived section that still shows its own unsettled debt, with its
expenses viewable via Activity's off-by-default Archived filter; a debt-simplification
toggle that switches between the minimal transfer set and raw pairwise balances,
detailed balances, **move expenses** between/into/out of groups via a guided
re-split), expenses (all five split methods + **scoped settlements** — both ways (settle what
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

Mirrored task runners live in [`scripts/`](scripts/) — `dev.sh` (bash) and
`dev.ps1` (PowerShell), behaviorally identical. Each task tees combined output
to `scripts/logs/<task>.log`.

```sh
./scripts/dev.sh build vet test cov   # fast post-change loop
./scripts/dev.sh vuln                 # govulncheck (report-only)
./scripts/dev.sh image                # build the distroless image (needs Docker)
./scripts/dev.sh trivy                # scan the built image for CVEs (report-only)
./scripts/dev.sh scan                 # vuln + image + trivy
./scripts/dev.sh full                 # all + scan
```

Override the image tag with `IMAGE_NAME` (default `gosplit:dev`). `trivy` runs
as a container (`aquasec/trivy`, override with `TRIVY_IMAGE`) — only Docker is
required, no local Trivy install.

GitHub Actions mirror these gates:

- **`.github/workflows/ci.yml`** — on push/PR: `build vet test cov` (fatal) +
  `govulncheck` (report-only), uploading the coverage profile.
- **`.github/workflows/release.yml`** — on a published release (and `v*` tags for
  a dry run): the same gates, then build the image, Trivy-scan it, and push
  `:<tag>` + `:latest` to GHCR.

Correctness gates (build/vet/test) are fatal; CVE/image scans are report-only —
raise Trivy's `exit-code` or the `vuln` handling if you want them blocking.

## License

MIT (matching the upstream project's spirit).
