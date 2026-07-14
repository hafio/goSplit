# Graph Report - gosplit  (2026-07-14)

## Corpus Check
- 117 files · ~68,740 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1166 nodes · 2576 edges · 99 communities (56 shown, 43 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 396 edges (avg confidence: 0.77)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `0fb5b44d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- User Service, Store & Auth
- HTTP Page & Settlement Handlers
- Expense Handlers & Money
- Server Wiring & Auth Middleware
- Web Templates (Layout/Auth)
- Config, Mail & Bootstrap
- Bank Sync (Plaid)
- HTTP Integration Tests
- Balance Simplify & Golden Tests
- Split Engine
- Recurring Expenses
- README Feature Overview
- UI Refresh Plan (Core)
- Group Store Queries
- Web Templates (Groups/Activity)
- Service Layer Tests
- Postgres Schema Migration
- SQLite Schema Migration
- Spec: Tech Stack & Features
- Dev Script (bash)
- Auth Page Handlers
- Expense Service Logic
- Currency Rate Providers
- Web Templates (Friends/Balances)
- Internationalization (i18n)
- Web Helpers & Categories
- Web Templates (Forms/Admin)
- ID/Time Helpers & Avatar
- Expense Store Writes
- Session Store & Models
- Spec: Data Model & Balances
- CI/CD Pipelines
- Theme Color System
- Spec: Auth & Entities
- Deployment & Compose
- Currency Conversion Math
- Spec: Split Engine & Currency
- Dev Script (PowerShell)
- Expense Notifications
- Background Scheduler
- Expense Query & Filtering
- Expense Form JS
- Push Subscription Store
- Splitwise Import Handlers
- Cached Rate Store
- Icon Sprite Sheet
- Web Push JS
- Spec: Testing & Milestones
- Project Conventions (CLAUDE.md)
- Recurring Handler Types
- Store Insert Helper
- Bank Connect JS
- App Icon & Branding
- Bank Handlers
- Service Worker JS
- htmx Library
- Go Module Definition
- Filter Bar Partial
- Filtered activity feed
- Bank sync (Plaid)
- chi router
- Debt simplification (min-cash-flow)
- Expenses
- Expense/activity filtering
- Friends
- Go 1.26+
- go:embed templates + assets
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
- ValidTheme
- TestTemplateKeysResolve

## God Nodes (most connected - your core abstractions)
1. `ctxTimeout()` - 59 edges
2. `Expense` - 35 edges
3. `User` - 33 edges
4. `body()` - 30 edges
5. `newHarness()` - 29 edges
6. `He()` - 29 edges
7. `Server` - 28 edges
8. `atoi64()` - 26 edges
9. `newTestService()` - 25 edges
10. `t()` - 25 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewManager()`  [INFERRED]
  cmd/gosplit/main.go → internal/auth/middleware.go
- `run()` --calls--> `NewRenderer()`  [INFERRED]
  cmd/gosplit/main.go → internal/web/view.go
- `run()` --calls--> `Open()`  [INFERRED]
  cmd/gosplit/main.go → internal/store/store.go
- `Postgres db service (optional, commented)` --conceptually_related_to--> `PostgreSQL`  [INFERRED]
  docker-compose.yml → README.md
- `Trivy image scan (gating)` --conceptually_related_to--> `dev.sh / dev.ps1 task runners`  [INFERRED]
  .github/workflows/release.yml → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]

## Communities (99 total, 43 thin omitted)

### Community 0 - "User Service, Store & Auth"
Cohesion: 0.17
Nodes (12): HashPassword(), RandomToken(), T, TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword() (+4 more)

### Community 1 - "HTTP Page & Settlement Handlers"
Cohesion: 0.06
Nodes (41): CancelFunc, balanceRow, currencyNet, expenseRow, friendRow, groupRow, monthGroup, nameCache (+33 more)

### Community 2 - "Expense Handlers & Money"
Cohesion: 0.08
Nodes (34): candidate, expenseDetailView, participantView, absInt64(), Server, Request, ResponseWriter, displayName() (+26 more)

### Community 3 - "Server Wiring & Auth Middleware"
Cohesion: 0.07
Nodes (32): ctxKey, Manager, HandlerFunc, CSRFFrom(), Context, Handler, Request, ResponseWriter (+24 more)

### Community 4 - "Web Templates (Layout/Auth)"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Config, Mail & Bootstrap"
Cohesion: 0.06
Nodes (35): Config, Engine, getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList(), Duration (+27 more)

### Community 6 - "Bank Sync (Plaid)"
Cohesion: 0.12
Nodes (11): Disabled, PlaidProvider, Provider, Transaction, Client, Context, New(), Context (+3 more)

### Community 7 - "HTTP Integration Tests"
Cohesion: 0.07
Nodes (51): harness, T, TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), T, TestExpenseNoteAndCollapse() (+43 more)

### Community 8 - "Balance Simplify & Golden Tests"
Cohesion: 0.07
Nodes (37): Transfer, main(), parseLogLevel(), run(), DB, Simplify(), T, netAfter() (+29 more)

### Community 9 - "Split Engine"
Cohesion: 0.23
Nodes (22): Compute(), computeShares(), DefaultSplitAllowed(), distributeRemainder(), finalize(), gcd(), seededOrder(), splitmix64() (+14 more)

### Community 10 - "Recurring Expenses"
Cohesion: 0.17
Nodes (10): Context, Service, toISO(), todayISO(), NullString, Context, Store, scanRecurrence() (+2 more)

### Community 11 - "README Feature Overview"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "UI Refresh Plan (Core)"
Cohesion: 0.09
Nodes (22): A1. Fingerprinted asset URLs — `internal/web/view.go`, A2. Templates use `{{asset}}` — 14 files in `internal/web/templates/`, A3. Router: compression + static routes outside auth — `internal/httpapp/server.go`, A4. Service worker: cache-first for /static — `internal/web/assets/sw.js`, A5. Tests for Phase A, B1. Vendor htmx, B2. Enable boost — `internal/web/templates/layout.html`, B3. Click feedback — `internal/web/assets/app.css` (+14 more)

### Community 13 - "Group Store Queries"
Cohesion: 0.08
Nodes (17): Context, Request, Service, Context, Store, prefixCols(), trimSpace(), Context (+9 more)

### Community 14 - "Web Templates (Groups/Activity)"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "Service Layer Tests"
Cohesion: 0.10
Nodes (35): T, TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), countRows(), Service, T, mkExp(), netByUser() (+27 more)

### Community 16 - "Postgres Schema Migration"
Cohesion: 0.32
Nodes (5): Context, Store, scanExpense(), Expense, ExpenseFilter

### Community 17 - "SQLite Schema Migration"
Cohesion: 0.24
Nodes (9): Context, NullInt64, NullString, Service, movable(), nullStr(), orDefault(), sameGroup() (+1 more)

### Community 19 - "Dev Script (bash)"
Cohesion: 0.42
Nodes (18): dev.sh script, die(), ok(), run(), step(), task_build(), task_test(), task_vet() (+10 more)

### Community 20 - "Auth Page Handlers"
Cohesion: 0.29
Nodes (4): Server, Request, ResponseWriter, safeNext()

### Community 21 - "Expense Service Logic"
Cohesion: 0.14
Nodes (13): FromNow(), Duration, nowISO(), Context, Duration, Store, Context, Duration (+5 more)

### Community 22 - "Currency Rate Providers"
Cohesion: 0.19
Nodes (8): FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, Provider, defaultClient(), Client, Context, NewProvider()

### Community 23 - "Web Templates (Friends/Balances)"
Cohesion: 0.21
Nodes (8): BuildLikePattern(), inPlaceholders(), NewUUID(), ExpenseParticipant, GroupScope, HistoricalBatch, VerificationToken, Tx

### Community 24 - "Internationalization (i18n)"
Cohesion: 0.23
Nodes (11): Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), T, load(), TestDetect() (+3 more)

### Community 25 - "Web Helpers & Categories"
Cohesion: 0.19
Nodes (13): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), T, TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange() (+5 more)

### Community 26 - "Web Templates (Forms/Admin)"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 31 - "CI/CD Pipelines"
Cohesion: 0.24
Nodes (12): build / vet / test / coverage step, gates job, govulncheck (report-only), CI workflow, Upload coverage artifact, Distroless image build, Push image to GHCR, image job (+4 more)

### Community 32 - "Theme Color System"
Cohesion: 0.32
Nodes (6): buildHistoricalCSV(), dateOnly(), Context, Service, sortedInt64Keys(), sortedStrKeys()

### Community 34 - "Deployment & Compose"
Cohesion: 0.12
Nodes (16): Postgres db service (optional, commented), SQLite default (no DB container), Architecture, Balances are derived, never stored, Configuration, Development & CI, Docker (SQLite, no DB container), Features (+8 more)

### Community 35 - "Currency Conversion Math"
Cohesion: 0.33
Nodes (8): Convert(), decimals(), ratPow10(), roundRat(), T, TestConvert(), TestConvertErrors(), Rat

### Community 37 - "Dev Script (PowerShell)"
Cohesion: 0.56
Nodes (7): Die(), Ok(), Run(), Step(), Task-Build(), Task-Test(), Task-Vet()

### Community 38 - "Expense Notifications"
Cohesion: 0.60
Nodes (3): FormatWithCode(), Context, Service

### Community 39 - "Background Scheduler"
Cohesion: 0.39
Nodes (5): Context, Duration, Service, New(), Scheduler

### Community 41 - "Expense Form JS"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 44 - "Push Subscription Store"
Cohesion: 0.38
Nodes (3): Context, Store, PushSubscription

### Community 47 - "Splitwise Import Handlers"
Cohesion: 0.60
Nodes (3): Server, Request, ResponseWriter

### Community 50 - "Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "Web Push JS"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 55 - "Project Conventions (CLAUDE.md)"
Cohesion: 0.50
Nodes (3): Build, Verify, Test, graphify, Project Conventions for Claude

### Community 59 - "App Icon & Branding"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 80 - "htmx Library"
Cohesion: 0.08
Nodes (104): $(), a(), Ae(), an(), at(), B(), Be(), bn() (+96 more)

### Community 129 - "ValidTheme"
Cohesion: 0.31
Nodes (9): T, TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme() (+1 more)

## Knowledge Gaps
- **124 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+119 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **43 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `User` connect `Group Store Queries` to `User Service, Store & Auth`, `HTTP Page & Settlement Handlers`, `Expense Handlers & Money`, `Server Wiring & Auth Middleware`, `Config, Mail & Bootstrap`, `Bank Sync (Plaid)`, `Recurring Expenses`, `Web Templates (Friends/Balances)`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Why does `New()` connect `Config, Mail & Bootstrap` to `Theme Color System`, `User Service, Store & Auth`, `Bank Sync (Plaid)`, `Recurring Expenses`, `Service Layer Tests`, `SQLite Schema Migration`, `Currency Rate Providers`?**
  _High betweenness centrality (0.113) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `HTTP Page & Settlement Handlers` to `Expense Handlers & Money`, `Server Wiring & Auth Middleware`, `Auth Page Handlers`, `Splitwise Import Handlers`?**
  _High betweenness centrality (0.082) - this node is a cross-community bridge._
- **Are the 55 inferred relationships involving `ctxTimeout()` (e.g. with `.handleActivity()` and `.handleAdminCreate()`) actually correct?**
  _`ctxTimeout()` has 55 INFERRED edges - model-reasoned connections that need verification._
- **Are the 22 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 22 INFERRED edges - model-reasoned connections that need verification._
- **Are the 21 inferred relationships involving `newHarness()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`newHarness()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _125 weakly-connected nodes found - possible documentation gaps or missing edges._