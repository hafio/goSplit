# Graph Report - gosplit  (2026-07-31)

## Corpus Check
- 136 files · ~93,935 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1412 nodes · 3472 edges · 115 communities (69 shown, 46 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 665 edges (avg confidence: 0.78)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `fe08542c`
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
- Web Templates (Groups/Activity)
- Service Layer Tests
- handlers_pages.go
- Spec: Tech Stack & Features
- Dev Script (bash)
- Auth Page Handlers
- Expense Service Logic
- Currency Rate Providers
- Internationalization (i18n)
- Web Helpers & Categories
- Web Templates (Forms/Admin)
- ID/Time Helpers & Avatar
- Expense Store Writes
- Session Store & Models
- Spec: Data Model & Balances
- CI/CD Pipelines
- Spec: Auth & Entities
- Deployment & Compose
- Currency Conversion Math
- Spec: Split Engine & Currency
- Dev Script (PowerShell)
- .CreateConversionExact
- Background Scheduler
- Expense Form JS
- nowISO
- Push Subscription Store
- .MoveExpense
- Server
- Splitwise Import Handlers
- New
- Cached Rate Store
- Icon Sprite Sheet
- Web Push JS
- Spec: Testing & Milestones
- .buildBatches
- body
- .PutBankData
- Recurring Handler Types
- Store Insert Helper
- Bank Connect JS
- App Icon & Branding
- Bank Handlers
- New
- Transaction
- seedDirectExpense
- Service Worker JS
- .findOrCreateUser
- htmx Library
- Go Module Definition
- TestAdminSetPasswordRevokesTargetSession
- TestExpenseFormRendersComponents
- TestConvertExactBothAmounts
- TestThemeColorUpdate
- TestExpenseNoteAndCollapse
- TestFilterChipsRender
- TestGroupDetailPolish
- TestListsRenderComponents
- atoi64
- load
- .handlePushSubscribe
- .handleRecurringDelete
- TestCov3BankConvertRedirect
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
- TestTemplateKeysResolve

## God Nodes (most connected - your core abstractions)
1. `body()` - 84 edges
2. `newHarness()` - 73 edges
3. `ctxTimeout()` - 63 edges
4. `newTestService()` - 57 edges
5. `User` - 39 edges
6. `Expense` - 36 edges
7. `openTestStore()` - 32 edges
8. `covUser()` - 30 edges
9. `New()` - 29 edges
10. `He()` - 29 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewManager()`  [INFERRED]
  cmd/gosplit/main.go → internal/auth/middleware.go
- `run()` --calls--> `NewRenderer()`  [INFERRED]
  cmd/gosplit/main.go → internal/web/view.go
- `run()` --calls--> `Open()`  [INFERRED]
  cmd/gosplit/main.go → internal/store/store.go
- `Trivy image scan (gating)` --conceptually_related_to--> `dev.sh / dev.ps1 task runners`  [INFERRED]
  .github/workflows/release.yml → README.md
- `app service` --conceptually_related_to--> `SplitPro (Go rebuild)`  [INFERRED]
  docker-compose.yml → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]

## Communities (115 total, 46 thin omitted)

### Community 0 - "User Service, Store & Auth"
Cohesion: 0.54
Nodes (3): Server, Request, ResponseWriter

### Community 1 - "HTTP Page & Settlement Handlers"
Cohesion: 0.19
Nodes (13): candidate, expenseDetailView, participantView, displayName(), Context, Server, Request, ResponseWriter (+5 more)

### Community 2 - "Expense Handlers & Money"
Cohesion: 0.25
Nodes (5): CancelFunc, Server, Request, ResponseWriter, ctxTimeout()

### Community 3 - "Server Wiring & Auth Middleware"
Cohesion: 0.09
Nodes (47): ctxKey, Manager, HandlerFunc, CSRFFrom(), Context, Handler, Request, ResponseWriter (+39 more)

### Community 4 - "Web Templates (Layout/Auth)"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Config, Mail & Bootstrap"
Cohesion: 0.09
Nodes (38): Config, Engine, getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList(), Duration (+30 more)

### Community 6 - "Bank Sync (Plaid)"
Cohesion: 0.11
Nodes (20): Disabled, PlaidProvider, Provider, roundTripFunc, Client, Context, New(), Request (+12 more)

### Community 7 - "HTTP Integration Tests"
Cohesion: 0.23
Nodes (25): body(), covNoRedirect(), T, TestAdminToggle(), TestBankPageAndConvertNotFound(), TestCollapsePreviewPages(), TestConvertPageRendersAndNotFound(), TestConvertRejectsBadInput() (+17 more)

### Community 8 - "Balance Simplify & Golden Tests"
Cohesion: 0.07
Nodes (57): Transfer, Simplify(), T, netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), T, TestUserNetByExpense() (+49 more)

### Community 9 - "Split Engine"
Cohesion: 0.12
Nodes (31): Context, NullInt64, NullString, Service, movable(), nullStr(), orDefault(), sameGroup() (+23 more)

### Community 10 - "Recurring Expenses"
Cohesion: 0.05
Nodes (32): Transaction, Context, Service, itoa(), Context, Service, Client, Context (+24 more)

### Community 11 - "README Feature Overview"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 14 - "Web Templates (Groups/Activity)"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "Service Layer Tests"
Cohesion: 0.06
Nodes (75): T, TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), countRows(), Service, T, mkExp(), netByUser() (+67 more)

### Community 17 - "handlers_pages.go"
Cohesion: 0.23
Nodes (8): friendRow, friendSettlePrefill(), Context, Store, scanBalances(), NullInt64, Balance, CumulatedBalance

### Community 19 - "Dev Script (bash)"
Cohesion: 0.42
Nodes (18): dev.sh script, die(), ok(), run(), step(), task_build(), task_test(), task_vet() (+10 more)

### Community 20 - "Auth Page Handlers"
Cohesion: 0.05
Nodes (29): HashPassword(), RandomToken(), T, TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword() (+21 more)

### Community 21 - "Expense Service Logic"
Cohesion: 0.05
Nodes (41): FormatWithCode(), buildHistoricalCSV(), dateOnly(), Context, Service, sortedInt64Keys(), sortedStrKeys(), Context (+33 more)

### Community 22 - "Currency Rate Providers"
Cohesion: 0.11
Nodes (32): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, Provider, defaultClient(), Client, Context (+24 more)

### Community 24 - "Internationalization (i18n)"
Cohesion: 0.19
Nodes (15): avatarColor(), categories(), categoryEmoji(), currencyCodes(), firstAlnum(), initials(), methodGlyph(), T (+7 more)

### Community 25 - "Web Helpers & Categories"
Cohesion: 0.15
Nodes (18): expenseRow, monthGroup, nameCache, T, TestActivityFeedShowAll(), TestFeedDateParts(), TestGroupByMonth(), applyFeedLimit() (+10 more)

### Community 26 - "Web Templates (Forms/Admin)"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 31 - "CI/CD Pipelines"
Cohesion: 0.38
Nodes (7): Distroless image build, Push image to GHCR, image job, Release workflow, Trivy image scan (gating), verify job, dev.sh / dev.ps1 task runners

### Community 34 - "Deployment & Compose"
Cohesion: 0.67
Nodes (3): Postgres db service (optional, commented), SQLite default (no DB container), SQLite (pure-Go modernc.org/sqlite)

### Community 35 - "Currency Conversion Math"
Cohesion: 0.33
Nodes (8): Convert(), decimals(), ratPow10(), roundRat(), T, TestConvert(), TestConvertErrors(), Rat

### Community 37 - "Dev Script (PowerShell)"
Cohesion: 0.56
Nodes (7): Die(), Ok(), Run(), Step(), Task-Build(), Task-Test(), Task-Vet()

### Community 38 - ".CreateConversionExact"
Cohesion: 0.19
Nodes (14): absInt64(), Server, Request, ResponseWriter, decimals(), Format(), Parse(), pow10() (+6 more)

### Community 39 - "Background Scheduler"
Cohesion: 0.12
Nodes (22): main(), parseLogLevel(), run(), DB, Context, Duration, Service, New() (+14 more)

### Community 41 - "Expense Form JS"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 43 - "nowISO"
Cohesion: 0.25
Nodes (21): assertNoSettlement(), bothRegistered(), chainHarness(), NullInt64, T, settlementGroups(), TestExpenseDeleteRejectsNonEditor(), TestFriendSettleFromCreditor() (+13 more)

### Community 44 - "Push Subscription Store"
Cohesion: 0.38
Nodes (3): Context, Store, PushSubscription

### Community 45 - ".MoveExpense"
Cohesion: 0.30
Nodes (14): covRenderer(), T, TestCovAsset(), TestCovAssetsHandlerCacheControl(), TestCovCSVRows(), TestCovCurrencyCodes(), TestCovFuncsAbs64(), TestCovFuncsDict() (+6 more)

### Community 46 - "Server"
Cohesion: 0.29
Nodes (4): Server, Request, ResponseWriter, safeNext()

### Community 47 - "Splitwise Import Handlers"
Cohesion: 0.60
Nodes (3): Server, Request, ResponseWriter

### Community 48 - "New"
Cohesion: 0.31
Nodes (9): T, TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme() (+1 more)

### Community 50 - "Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "Web Push JS"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - ".buildBatches"
Cohesion: 0.13
Nodes (10): T, TestQueryWithout(), csvRows(), Handler, ResponseWriter, NewRenderer(), queryWithout(), Template (+2 more)

### Community 54 - "body"
Cohesion: 0.26
Nodes (18): newHarness(), covPostJSON(), Response, T, TestCov2BankDisabledErrors(), TestCov2ConvertPagePreselectsBalance(), TestCov2ConvertSuccess(), TestCov2FriendCollapse() (+10 more)

### Community 55 - ".PutBankData"
Cohesion: 0.24
Nodes (13): harness, Client, Response, Store, T, Values, TestCSRFRejected(), TestFullFlow() (+5 more)

### Community 59 - "App Icon & Branding"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - "New"
Cohesion: 0.27
Nodes (10): balanceRow, currencyNet, groupRow, settlementRow, Context, max64(), min64(), rawSettlementRows() (+2 more)

### Community 62 - "Transaction"
Cohesion: 0.33
Nodes (4): Bundle, Load(), parseAcceptLanguage(), parseQ()

### Community 64 - "seedDirectExpense"
Cohesion: 0.44
Nodes (9): T, Values, seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP() (+1 more)

### Community 66 - ".findOrCreateUser"
Cohesion: 0.25
Nodes (10): New(), T, TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+2 more)

### Community 80 - "htmx Library"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

### Community 82 - "TestAdminSetPasswordRevokesTargetSession"
Cohesion: 0.53
Nodes (5): T, TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement()

### Community 84 - "TestExpenseFormRendersComponents"
Cohesion: 0.60
Nodes (4): T, TestExpenseFormRendersComponents(), TestExpenseFormRoundTripsMethods(), TestExpenseFormTargetSelector()

### Community 85 - "TestConvertExactBothAmounts"
Cohesion: 0.67
Nodes (3): T, TestConvertExactBothAmounts(), TestRateEndpoint()

### Community 86 - "TestThemeColorUpdate"
Cohesion: 0.67
Nodes (3): firstTag(), T, TestThemeColorUpdate()

### Community 91 - "atoi64"
Cohesion: 0.57
Nodes (4): Server, Request, ResponseWriter, atoi64()

### Community 92 - "load"
Cohesion: 0.57
Nodes (7): firstOther(), T, load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate()

### Community 93 - ".handlePushSubscribe"
Cohesion: 0.57
Nodes (3): Server, Request, ResponseWriter

### Community 94 - ".handleRecurringDelete"
Cohesion: 0.60
Nodes (3): Server, Request, ResponseWriter

### Community 95 - "TestCov3BankConvertRedirect"
Cohesion: 0.70
Nodes (4): covSeedBankTx(), T, TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

## Knowledge Gaps
- **95 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+90 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **46 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `newHarness()` connect `body` to `TestGroupDetailPolish`, `seedDirectExpense`, `Server Wiring & Auth Middleware`, `TestListsRenderComponents`, `Background Scheduler`, `HTTP Integration Tests`, `nowISO`, `TestAdminSetPasswordRevokesTargetSession`, `TestExpenseFormRendersComponents`, `TestConvertExactBothAmounts`, `.PutBankData`, `TestExpenseNoteAndCollapse`, `TestFilterChipsRender`, `Web Helpers & Categories`, `.buildBatches`, `TestThemeColorUpdate`, `TestCov3BankConvertRedirect`?**
  _High betweenness centrality (0.149) - this node is a cross-community bridge._
- **Why does `New()` connect `Recurring Expenses` to `Config, Mail & Bootstrap`, `Split Engine`, `Service Layer Tests`, `Auth Page Handlers`, `Expense Service Logic`, `Currency Rate Providers`?**
  _High betweenness centrality (0.135) - this node is a cross-community bridge._
- **Why does `Open()` connect `Background Scheduler` to `Server Wiring & Auth Middleware`, `Config, Mail & Bootstrap`, `Balance Simplify & Golden Tests`, `Service Layer Tests`, `body`?**
  _High betweenness centrality (0.127) - this node is a cross-community bridge._
- **Are the 74 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 74 INFERRED edges - model-reasoned connections that need verification._
- **Are the 64 inferred relationships involving `newHarness()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`newHarness()` has 64 INFERRED edges - model-reasoned connections that need verification._
- **Are the 59 inferred relationships involving `ctxTimeout()` (e.g. with `.handleActivity()` and `.handleAdminCreate()`) actually correct?**
  _`ctxTimeout()` has 59 INFERRED edges - model-reasoned connections that need verification._
- **Are the 46 inferred relationships involving `newTestService()` (e.g. with `TestAdminCreateUser()` and `TestAdminSetPasswordRevokesSessions()`) actually correct?**
  _`newTestService()` has 46 INFERRED edges - model-reasoned connections that need verification._