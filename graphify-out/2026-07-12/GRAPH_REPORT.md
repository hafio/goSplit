# Graph Report - gosplit  (2026-07-12)

## Corpus Check
- 77 files · ~44,328 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 893 nodes · 1651 edges · 134 communities (45 shown, 89 thin omitted)
- Extraction: 86% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 228 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Expense Store & Services
- HTTP Server & Page Handlers
- Expense Handlers & Money Parsing
- User Service & Store
- Config, Mail & Bootstrap
- i18n Rendering & HTTP Tests
- Web Templates (Groups/Friends)
- Web Templates (Auth/Layout)
- Auth Middleware & Sessions
- Bank Sync (Plaid)
- Split Engine
- Group Store Queries
- Balance Simplify & Golden Tests
- Recurring Expenses
- Service Layer Tests
- Postgres Schema Migration
- SQLite Schema Migration
- README Feature Overview
- Spec: Tech Stack & Features
- Dev Script (bash)
- Auth Page Handlers
- Spec: Data Model & Balances
- Currency Rate Providers
- Spec: Auth, Push & Entities
- CI/CD Pipelines
- Deployment & Compose
- Currency Conversion Math
- Spec: Split Engine & Currency
- Dev Script (PowerShell)
- Background Scheduler
- Push Subscription Store
- Splitwise Import Handlers
- Cached Rate Store
- Web Push JS
- Project Conventions (CLAUDE.md)
- Recurring Expense Handlers
- Store Insert Helper
- Bank Connect JS
- App Icon & Branding
- Bank Handlers
- Postgres Recurrence Migration
- SQLite Recurrence Migration
- Service Worker JS
- Filter Bar Partial
- Go Module Definition
- chi router
- database/sql query layer (as-built)
- Debt simplification (min-cash-flow)
- Expenses
- Expense/activity filtering
- Friends
- Go 1.26+
- go:embed templates + assets
- Golden Scenario regression fixture
- Groups
- Hand-rolled sessions (argon2id, CSRF, secure cookies)
- Hand-written CSS (as-built)
- stdlib html/template views (as-built)
- htmx progressive enhancement
- Localization (i18n JSON catalogs)
- Money as int64 minor units
- Move expenses between groups
- Installable PWA
- Recurring expenses (cron)
- Scheduler (DB leader lock)
- Settlements / settle-up
- Single static Go binary
- Split engine (internal/split)
- Splitwise import
- Web Push notifications (VAPID)
- ADJUSTMENT split
- Alpine.js
- Alternatives considered (Phoenix, Rust/Axum)
- Backend-heavy, GUI-light thin client
- BalanceView (SQL VIEW)
- Balances are derived, not stored
- caarlos0/env typed config
- CachedBankData entity
- CachedCurrencyRate entity
- Chosen stack (Go+htmx+templ+Tailwind+Postgres)
- CURRENCY_CONVERSION
- Data model (preserve current schema)
- Debt simplification (min-cash-flow)
- Group/FriendDefaultSplit
- Deployment (distroless + compose)
- EQUAL split
- EXACT split
- Expense entity
- ExpenseParticipant entity
- ExpenseRecurrence entity
- Filtering (§5.2)
- nicksnyder/go-i18n
- wneessen/go-mail
- Golden Scenario
- goose migrations
- Group entity
- GroupUser (membership join)
- disintegration/imaging
- Build milestones M0-M11
- Money is never a float
- Move expense (§5.2)
- Parity first (drop OAuth/OIDC)
- PERCENTAGE split
- plaid/plaid-go
- PushNotification entity
- Python reference generator
- robfig/cron scheduler
- SchedulerLock entity
- Session entity
- SETTLEMENT (settle-up)
- SHARE split
- Split engine (§5.1)
- sqlc (type-safe SQL)
- Tailwind CSS (standalone CLI)
- templ (type-safe HTML views)
- User entity
- VerificationToken entity
- webpush-go (VAPID)
- Zero-sum signed participant convention

## God Nodes (most connected - your core abstractions)
1. `ctxTimeout()` - 51 edges
2. `User` - 31 edges
3. `Expense` - 25 edges
4. `Server` - 23 edges
5. `New()` - 20 edges
6. `atoi64()` - 17 edges
7. `Server` - 16 edges
8. `Config` - 15 edges
9. `Base Layout` - 15 edges
10. `nowISO()` - 14 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewManager()`  [INFERRED]
  cmd/gosplit/main.go → internal/auth/middleware.go
- `run()` --calls--> `NewRenderer()`  [INFERRED]
  cmd/gosplit/main.go → internal/web/view.go
- `run()` --calls--> `Open()`  [INFERRED]
  cmd/gosplit/main.go → internal/store/store.go
- `Postgres db service (optional, commented)` --conceptually_related_to--> `PostgreSQL`  [INFERRED]
  docker-compose.yml → README.md
- `app service` --conceptually_related_to--> `SplitPro (Go rebuild)`  [INFERRED]
  docker-compose.yml → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]

## Communities (134 total, 89 thin omitted)

### Community 0 - "Expense Store & Services"
Cohesion: 0.06
Nodes (38): Context, Service, Context, NullInt64, NullString, Service, movable(), nullInt() (+30 more)

### Community 1 - "HTTP Server & Page Handlers"
Cohesion: 0.08
Nodes (29): CancelFunc, balanceRow, currencyNet, groupRow, settlementRow, Server, Request, ResponseWriter (+21 more)

### Community 2 - "Expense Handlers & Money Parsing"
Cohesion: 0.06
Nodes (39): candidate, expenseDetailView, expenseRow, friendRow, nameCache, participantView, displayName(), Context (+31 more)

### Community 3 - "User Service & Store"
Cohesion: 0.06
Nodes (32): HashPassword(), RandomToken(), T, TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword() (+24 more)

### Community 4 - "Config, Mail & Bootstrap"
Cohesion: 0.08
Nodes (26): main(), run(), Config, Engine, DB, getEnv(), getEnvBool(), getEnvDuration() (+18 more)

### Community 5 - "i18n Rendering & HTTP Tests"
Cohesion: 0.09
Nodes (27): harness, Bundle, body(), Client, T, newHarness(), TestCSRFRejected(), TestFullFlow() (+19 more)

### Community 6 - "Web Templates (Groups/Friends)"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 7 - "Web Templates (Auth/Layout)"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 8 - "Auth Middleware & Sessions"
Cohesion: 0.12
Nodes (21): ctxKey, Manager, HandlerFunc, CSRFFrom(), Context, Handler, Request, ResponseWriter (+13 more)

### Community 9 - "Bank Sync (Plaid)"
Cohesion: 0.12
Nodes (11): Disabled, PlaidProvider, Provider, Transaction, Client, Context, New(), Context (+3 more)

### Community 10 - "Split Engine"
Cohesion: 0.23
Nodes (22): Compute(), computeShares(), DefaultSplitAllowed(), distributeRemainder(), finalize(), gcd(), seededOrder(), splitmix64() (+14 more)

### Community 11 - "Group Store Queries"
Cohesion: 0.21
Nodes (6): prefixCols(), trimSpace(), Context, Store, scanGroup(), Group

### Community 12 - "Balance Simplify & Golden Tests"
Cohesion: 0.15
Nodes (18): Transfer, Simplify(), T, netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), T, TestBuildLikePattern() (+10 more)

### Community 13 - "Recurring Expenses"
Cohesion: 0.18
Nodes (9): Context, Service, toISO(), NullString, Context, Store, scanRecurrence(), ExpenseRecurrence (+1 more)

### Community 14 - "Service Layer Tests"
Cohesion: 0.22
Nodes (14): extractToken(), Context, Service, T, newTestService(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion() (+6 more)

### Community 15 - "Postgres Schema Migration"
Cohesion: 0.12
Nodes (15): 1. Guiding principles, 2. Chosen stack — "new ecosystem, lightweight, easy", 3. Current → new stack mapping (parity guide), 4.1 How balances are computed (important), 4. Data model (preserve current schema), 5.1 Split methods (all retained — exact behavior), 5.2 Filtering & moving expenses (NEW — beyond parity), 5. Feature inventory (must reach parity) (+7 more)

### Community 17 - "README Feature Overview"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 19 - "Dev Script (bash)"
Cohesion: 0.42
Nodes (17): dev.sh script, die(), ok(), run(), step(), task_build(), task_test(), task_vet() (+9 more)

### Community 20 - "Auth Page Handlers"
Cohesion: 0.29
Nodes (4): Server, Request, ResponseWriter, safeNext()

### Community 22 - "Currency Rate Providers"
Cohesion: 0.19
Nodes (8): FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, Provider, defaultClient(), Client, Context, NewProvider()

### Community 24 - "CI/CD Pipelines"
Cohesion: 0.22
Nodes (13): build / vet / test / coverage step, gates job, govulncheck (report-only), CI workflow, Upload coverage artifact, Distroless image build, Push image to GHCR, image job (+5 more)

### Community 25 - "Deployment & Compose"
Cohesion: 0.11
Nodes (17): Postgres db service (optional, commented), SQLite default (no DB container), Architecture, Balances are derived, never stored, Configuration, Development & CI, Docker (SQLite, no DB container), Features (+9 more)

### Community 26 - "Currency Conversion Math"
Cohesion: 0.33
Nodes (8): Convert(), decimals(), ratPow10(), roundRat(), T, TestConvert(), TestConvertErrors(), Rat

### Community 28 - "Dev Script (PowerShell)"
Cohesion: 0.56
Nodes (7): Die(), Ok(), Run(), Step(), Task-Build(), Task-Test(), Task-Vet()

### Community 29 - "Background Scheduler"
Cohesion: 0.39
Nodes (5): Context, Duration, Service, New(), Scheduler

### Community 30 - "Push Subscription Store"
Cohesion: 0.38
Nodes (3): Context, Store, PushSubscription

### Community 31 - "Splitwise Import Handlers"
Cohesion: 0.60
Nodes (3): Server, Request, ResponseWriter

### Community 33 - "Web Push JS"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 34 - "Project Conventions (CLAUDE.md)"
Cohesion: 0.50
Nodes (3): Build, Verify, Test, graphify, Project Conventions for Claude

### Community 38 - "App Icon & Branding"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

## Knowledge Gaps
- **157 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `bankTxRow`, `balanceRow`, `recurrenceRow` (+152 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **89 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `User` connect `User Service & Store` to `Expense Store & Services`, `Expense Handlers & Money Parsing`, `i18n Rendering & HTTP Tests`, `Auth Middleware & Sessions`, `Bank Sync (Plaid)`, `Group Store Queries`, `Recurring Expenses`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Why does `New()` connect `User Service & Store` to `Expense Store & Services`, `Config, Mail & Bootstrap`, `Bank Sync (Plaid)`, `Recurring Expenses`, `Service Layer Tests`, `Currency Rate Providers`?**
  _High betweenness centrality (0.096) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `HTTP Server & Page Handlers` to `Auth Middleware & Sessions`, `Expense Handlers & Money Parsing`, `Auth Page Handlers`, `Splitwise Import Handlers`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Are the 47 inferred relationships involving `ctxTimeout()` (e.g. with `.handleActivity()` and `.handleAdmin()`) actually correct?**
  _`ctxTimeout()` has 47 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `bankTxRow` to the rest of the system?**
  _166 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Expense Store & Services` be split into smaller, more focused modules?**
  _Cohesion score 0.05981981981981982 - nodes in this community are weakly interconnected._
- **Should `HTTP Server & Page Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.07945566286215978 - nodes in this community are weakly interconnected._