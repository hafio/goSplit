# Graph Report - gosplit  (2026-07-13)

## Corpus Check
- 120 files · ~70,793 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1096 nodes · 2109 edges · 134 communities (59 shown, 75 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 334 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `91739360`
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
- UI Refresh Plan (Forms/Theme)
- Web Push Handlers
- Push Subscription Store
- Recurring Expense Handlers
- Verification Token Store
- Splitwise Import Handlers
- Currency Service
- Cached Rate Store
- Icon Sprite Sheet
- Web Push JS
- Spec: Testing & Milestones
- UI Refresh Plan (Theme Palette)
- Bank Data Store
- Project Conventions (CLAUDE.md)
- Recurring Handler Types
- Store Insert Helper
- Bank Connect JS
- App Icon & Branding
- Bank Handlers
- Postgres Recurrence Migration
- Postgres Theme Migration
- SQLite Recurrence Migration
- SQLite Theme Migration
- Service Worker JS
- Convert Handler
- Go Module Definition
- Phase 1: Foundation (icons, helpers, CSS components)
- Phase 2: Theme Color (user-selectable, burgundy default)
- Phase 3: Lists & Navigation
- Phase 4: Expense Feed (month groups, category icons, you lent/borrowed)
- Phase 5: Add-Expense Form
- Phase 7: Group Page & Detail Polish
- Progressive Enhancement
- Semantic Amount Colors (--pos/--neg)
- Server-Rendered html/template Architecture
- Split Method Segmented Control
- SplitPro/Splitwise Interaction Language
- Stat Hero / Tiles
- Single Static Binary (go:embed Assets)
- Template Helper Funcs (initials/avatarColor/categoryEmoji/categories)
- Named Accent Theme Palette
- Theme Registry (theme.go)
- Theme Swatch Picker (Profile Appearance)
- Vanilla JS Enhancement (No Alpine)
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
- Plan: Input hover/focus highlight · Edit+Delete icons · Unify move & edit
- New
- ValidTheme
- .ImportFromSplitwise
- .CreateConversionExact
- .UpdateAvatar
- .AdminCreateUser

## God Nodes (most connected - your core abstractions)
1. `ctxTimeout()` - 59 edges
2. `Expense` - 34 edges
3. `User` - 33 edges
4. `Server` - 28 edges
5. `body()` - 28 edges
6. `newHarness()` - 27 edges
7. `atoi64()` - 26 edges
8. `newTestService()` - 25 edges
9. `New()` - 22 edges
10. `Config` - 16 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewManager()`  [INFERRED]
  cmd/gosplit/main.go → internal/auth/middleware.go
- `run()` --calls--> `Open()`  [INFERRED]
  cmd/gosplit/main.go → internal/store/store.go
- `run()` --calls--> `NewRenderer()`  [INFERRED]
  cmd/gosplit/main.go → internal/web/view.go
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

## Communities (134 total, 75 thin omitted)

### Community 0 - "User Service, Store & Auth"
Cohesion: 0.21
Nodes (10): HashPassword(), RandomToken(), T, TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword() (+2 more)

### Community 1 - "HTTP Page & Settlement Handlers"
Cohesion: 0.08
Nodes (29): CancelFunc, balanceRow, currencyNet, groupRow, settlementRow, Server, Request, ResponseWriter (+21 more)

### Community 2 - "Expense Handlers & Money"
Cohesion: 0.12
Nodes (21): candidate, expenseDetailView, friendRow, participantView, displayName(), Context, Server, Request (+13 more)

### Community 3 - "Server Wiring & Auth Middleware"
Cohesion: 0.11
Nodes (22): ctxKey, Manager, HandlerFunc, CSRFFrom(), Context, Handler, Request, ResponseWriter (+14 more)

### Community 4 - "Web Templates (Layout/Auth)"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Config, Mail & Bootstrap"
Cohesion: 0.08
Nodes (28): Config, Engine, DB, getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList() (+20 more)

### Community 6 - "Bank Sync (Plaid)"
Cohesion: 0.12
Nodes (11): Disabled, PlaidProvider, Provider, Transaction, Client, Context, New(), Context (+3 more)

### Community 7 - "HTTP Integration Tests"
Cohesion: 0.08
Nodes (45): harness, T, TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), T, TestExpenseNoteAndCollapse() (+37 more)

### Community 8 - "Balance Simplify & Golden Tests"
Cohesion: 0.09
Nodes (25): Transfer, Simplify(), T, netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), T, TestUserNetByExpense() (+17 more)

### Community 9 - "Split Engine"
Cohesion: 0.23
Nodes (22): Compute(), computeShares(), DefaultSplitAllowed(), distributeRemainder(), finalize(), gcd(), seededOrder(), splitmix64() (+14 more)

### Community 10 - "Recurring Expenses"
Cohesion: 0.18
Nodes (9): Context, Service, toISO(), NullString, Context, Store, scanRecurrence(), ExpenseRecurrence (+1 more)

### Community 11 - "README Feature Overview"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "UI Refresh Plan (Core)"
Cohesion: 0.11
Nodes (17): Avatar colors (initials circles), Category emoji map (stored value → emoji), Context, Design constants (single source of truth), Docs to keep in sync (same change as the code), GoSplit UI/UX Refresh — Implementation Plan, Icon sprite, Ledger (+9 more)

### Community 13 - "Group Store Queries"
Cohesion: 0.15
Nodes (7): Context, Store, Context, Store, orDefault(), scanUser(), User

### Community 14 - "Web Templates (Groups/Activity)"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "Service Layer Tests"
Cohesion: 0.10
Nodes (35): T, TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), countRows(), Service, T, mkExp(), netByUser() (+27 more)

### Community 16 - "Postgres Schema Migration"
Cohesion: 0.19
Nodes (13): expenseRow, monthGroup, nameCache, T, TestFeedDateParts(), TestGroupByMonth(), dateOnly(), feedDateParts() (+5 more)

### Community 17 - "SQLite Schema Migration"
Cohesion: 0.17
Nodes (11): Context, Decisions (confirmed with user — final), Existing primitives to reuse (do not reinvent), Files to touch, GoSplit: Admin Users + Conversion Overhaul — Implementation Plan, Ledger, Phase 1 — Admin: add user + set password, Phase 2 — Rates endpoint (JSON, validated) (+3 more)

### Community 19 - "Dev Script (bash)"
Cohesion: 0.42
Nodes (18): dev.sh script, die(), ok(), run(), step(), task_build(), task_test(), task_vet() (+10 more)

### Community 20 - "Auth Page Handlers"
Cohesion: 0.29
Nodes (4): Server, Request, ResponseWriter, safeNext()

### Community 21 - "Expense Service Logic"
Cohesion: 0.05
Nodes (44): buildHistoricalCSV(), dateOnly(), Context, Service, sortedInt64Keys(), sortedStrKeys(), Context, NullInt64 (+36 more)

### Community 22 - "Currency Rate Providers"
Cohesion: 0.19
Nodes (8): FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, Provider, defaultClient(), Client, Context, NewProvider()

### Community 24 - "Internationalization (i18n)"
Cohesion: 0.08
Nodes (24): main(), parseLogLevel(), run(), Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther() (+16 more)

### Community 25 - "Web Helpers & Categories"
Cohesion: 0.22
Nodes (11): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), T, TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+3 more)

### Community 26 - "Web Templates (Forms/Admin)"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 31 - "CI/CD Pipelines"
Cohesion: 0.24
Nodes (12): build / vet / test / coverage step, gates job, govulncheck (report-only), CI workflow, Upload coverage artifact, Distroless image build, Push image to GHCR, image job (+4 more)

### Community 32 - "Theme Color System"
Cohesion: 0.21
Nodes (6): prefixCols(), trimSpace(), Context, Store, scanGroup(), Group

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
Cohesion: 0.15
Nodes (17): absInt64(), Server, Request, ResponseWriter, decimals(), Format(), FormatWithCode(), Parse() (+9 more)

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

### Community 127 - "Plan: Input hover/focus highlight · Edit+Delete icons · Unify move & edit"
Cohesion: 0.18
Nodes (10): 3a. Service: ack only on group change, 3b. Handler: real edit page with pre-fill and `?target=` retarget, 3c. Template + JS: banner/ack on `GroupChanged`, retarget navigation, Context, Ledger, Phase 1 — Input hover/focus accent (CSS only), Phase 2 — Edit + Delete icons replace the "…" menu, Phase 3 — Unify move & edit (+2 more)

### Community 128 - "New"
Cohesion: 0.25
Nodes (7): Context, Service, Service, Store, New(), todayISO(), Provider

### Community 129 - "ValidTheme"
Cohesion: 0.31
Nodes (9): T, TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme() (+1 more)

### Community 130 - ".ImportFromSplitwise"
Cohesion: 0.39
Nodes (5): Client, Context, Service, swGet(), ImportResult

### Community 132 - ".UpdateAvatar"
Cohesion: 0.50
Nodes (3): Context, Request, Service

## Knowledge Gaps
- **170 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+165 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **75 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `User` connect `Group Store Queries` to `User Service, Store & Auth`, `New`, `Expense Handlers & Money`, `Server Wiring & Auth Middleware`, `.ImportFromSplitwise`, `.AdminCreateUser`, `Bank Sync (Plaid)`, `.UpdateAvatar`, `Theme Color System`, `Recurring Expenses`, `Expense Service Logic`, `Internationalization (i18n)`?**
  _High betweenness centrality (0.128) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `User Service, Store & Auth`, `.ImportFromSplitwise`, `.CreateConversionExact`, `Config, Mail & Bootstrap`, `Bank Sync (Plaid)`, `Recurring Expenses`, `Service Layer Tests`, `Expense Service Logic`, `Currency Rate Providers`?**
  _High betweenness centrality (0.117) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `HTTP Page & Settlement Handlers` to `Expense Handlers & Money`, `Server Wiring & Auth Middleware`, `Expense Notifications`, `Splitwise Import Handlers`, `Auth Page Handlers`?**
  _High betweenness centrality (0.099) - this node is a cross-community bridge._
- **Are the 55 inferred relationships involving `ctxTimeout()` (e.g. with `.handleActivity()` and `.handleAdminCreate()`) actually correct?**
  _`ctxTimeout()` has 55 INFERRED edges - model-reasoned connections that need verification._
- **Are the 21 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _171 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `HTTP Page & Settlement Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.08145131432802666 - nodes in this community are weakly interconnected._