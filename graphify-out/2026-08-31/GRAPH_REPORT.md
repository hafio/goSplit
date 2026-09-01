# Graph Report - gosplit  (2026-08-31)

## Corpus Check
- 148 files · ~109,956 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1387 nodes · 3761 edges · 111 communities (66 shown, 45 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 639 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `974fdff0`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- handlers_pages.go
- covUser
- net/http.Request
- NewManager
- Base Layout
- Config
- provider_test.go
- body
- openTestStore
- Method
- ExpenseRecurrence
- app service
- Group
- Service
- Group Detail Page
- newTestService
- nowISO
- Store
- database/sql query layer (as-built)
- dev.sh
- RandomToken
- Expense
- currency/zz_coverage_test.go
- nullInt
- Renderer
- context.Context
- convert.js
- collapseView
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- .buildBatches
- Auth (magic-link + password)
- SQLite default (no DB container)
- helpers.go
- Currency conversion (pluggable providers)
- dev.ps1
- Bundle
- service/split_inputs_test.go
- Transaction
- expense_form.js
- service/zz_coverage2_test.go
- settle_test.go
- Store
- testing.T
- New
- Convert
- mkExp
- PlaidProvider
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- ValidTheme
- .CreateConversionExact
- harness
- handlers_recurring.go
- NewRenderer
- bank.js
- GoSplit App Icon
- bankTxRow
- nameCache
- .serveAsset
- TestCov3BankConvertRedirect
- seedDirectExpense
- sw.js
- New
- htmx.min.js
- github.com/hafio/gosplit
- fakeRates
- TestThemeColorUpdate
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

## God Nodes (most connected - your core abstractions)
1. `body()` - 100 edges
2. `newHarness()` - 77 edges
3. `newTestService()` - 66 edges
4. `ctxTimeout()` - 63 edges
5. `Expense` - 45 edges
6. `User` - 43 edges
7. `covUser()` - 38 edges
8. `openTestStore()` - 38 edges
9. `atoi64()` - 32 edges
10. `Server` - 28 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `Load()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/config/config.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/mail/mail.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/service/service.go
- `run()` --calls--> `NewRenderer()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/web/view.go
- `app service` --conceptually_related_to--> `SplitPro (Go rebuild)`  [INFERRED]
  docker-compose.yml → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (111 total, 45 thin omitted)

### Community 0 - "handlers_pages.go"
Cohesion: 0.16
Nodes (15): Transfer, balanceRow, currencyNet, friendRow, groupRow, settlementRow, Simplify(), netAfter() (+7 more)

### Community 1 - "covUser"
Cohesion: 0.20
Nodes (17): covUser(), Service, TestAddExpenseDirectCreatesFriendship(), TestAddExpenseNonMemberRejected(), TestAddFriendByEmail(), TestAdminMagicLinkForUser(), TestArchiveDirectRejectsSelf(), TestArchiveGroupNonMember() (+9 more)

### Community 2 - "net/http.Request"
Cohesion: 0.06
Nodes (30): context.CancelFunc, net/http.Request, net/http.ResponseWriter, expenseRow, monthGroup, Server, Server, safeNext() (+22 more)

### Community 3 - "NewManager"
Cohesion: 0.07
Nodes (49): ctxKey, main(), parseLogLevel(), run(), wantsVersion(), database/sql.DB, log/slog.Level, net/http.Handler (+41 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Config"
Cohesion: 0.07
Nodes (36): crypto/tls.Config, net.Listener, net/smtp.Client, getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList() (+28 more)

### Community 6 - "provider_test.go"
Cohesion: 0.26
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 7 - "body"
Cohesion: 0.09
Nodes (49): net/http.Response, TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestConvertExactBothAmounts(), TestRateEndpoint(), TestFilterChipsRender(), TestGroupAddFriendByPicker(), TestGroupDetailPolish() (+41 more)

### Community 8 - "openTestStore"
Cohesion: 0.07
Nodes (52): TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit(), TestGroupMemberCountsAndNets(), TestListFriendExpensesFilters(), TestAdminUpdateUser(), TestListFriendExpensesIncludesGroups(), buildAmounts() (+44 more)

### Community 9 - "Method"
Cohesion: 0.06
Nodes (65): candidate, expenseDetailView, fieldError, participantView, fieldErrorf(), formatValue(), formVersion(), lineForMethod() (+57 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.17
Nodes (6): time.Time, Service, toISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "app service"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.18
Nodes (6): prefixCols(), trimSpace(), Store, scanGroup(), Group, TestCov2PureHelpers()

### Community 13 - "Service"
Cohesion: 0.23
Nodes (9): database/sql.NullInt64, database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullStr(), orDefault() (+1 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "newTestService"
Cohesion: 0.15
Nodes (16): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestCreateConversionExact(), TestAddFriendToGroup(), extractToken(), Service, newTestService(), TestAdminAutoPromotion() (+8 more)

### Community 16 - "nowISO"
Cohesion: 0.12
Nodes (11): time.Duration, Store, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store (+3 more)

### Community 17 - "Store"
Cohesion: 0.24
Nodes (5): friendSettlePrefill(), CumulatedBalance, Store, scanBalances(), Balance

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 21 - "Expense"
Cohesion: 0.13
Nodes (13): database/sql.Tx, FormatWithCode(), Service, BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, inPlaceholders() (+5 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.10
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "nullInt"
Cohesion: 0.28
Nodes (7): TestDeleteExpenseAuthorization(), nullInt(), TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted(), TestMoveExpenseUnauthorized()

### Community 24 - "Renderer"
Cohesion: 0.16
Nodes (6): html/template.Template, TestQueryWithout(), assetURL(), Renderer, ViewData, queryWithout()

### Community 25 - "context.Context"
Cohesion: 0.09
Nodes (13): context.Context, Service, Service, swGet(), Service, Store, Store, User (+5 more)

### Community 26 - "convert.js"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 32 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 34 - "SQLite default (no DB container)"
Cohesion: 0.67
Nodes (3): Postgres db service (optional, commented), SQLite default (no DB container), SQLite (pure-Go modernc.org/sqlite)

### Community 35 - "helpers.go"
Cohesion: 0.21
Nodes (11): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+3 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Bundle"
Cohesion: 0.27
Nodes (7): Bundle, firstOther(), load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate()

### Community 39 - "service/split_inputs_test.go"
Cohesion: 0.23
Nodes (11): generatedFrom(), Service, TestAddExpenseOmitsNonSplittingPayer(), TestAddExpenseRecordsTypedInputs(), TestGenerateOneCopiesWhenNoInputs(), TestGenerateOneReSplitsFromInputs(), TestMoveExpenseRejectsStaleVersion(), TestSettleRecordsNoInputs() (+3 more)

### Community 40 - "Transaction"
Cohesion: 0.18
Nodes (5): Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 42 - "service/zz_coverage2_test.go"
Cohesion: 0.18
Nodes (9): covMultipartReq(), TestCovBankConnectedFlow(), TestCovBankProviderErrors(), TestCovConversionErrors(), TestCovEmailParticipants(), TestCovInviteSendsPendingInvites(), TestCovPushGating(), TestCovUpdateAvatar() (+1 more)

### Community 43 - "settle_test.go"
Cohesion: 0.16
Nodes (24): settlementPath(), TestEditSettlementGroupMoveNeedsAck(), TestEditSettlementPageHasNoMethodPicker(), TestEditSettlementRejectsBadAmount(), TestEditSettlementUpdatesAmount(), assertNoSettlement(), bothRegistered(), chainHarness() (+16 more)

### Community 45 - "testing.T"
Cohesion: 0.10
Nodes (29): TestVersionDefault(), TestWantsVersion(), testing.T, TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseFormRendersComponents() (+21 more)

### Community 46 - "New"
Cohesion: 0.18
Nodes (9): New(), TestCovAuthErrorBranches(), TestCovDispatchDefaults(), TestCovEmailParticipantsSendError(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovPushEnabledSendError(), TestCovRecurrenceErrorBranches() (+1 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.44
Nodes (9): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+1 more)

### Community 49 - "PlaidProvider"
Cohesion: 0.17
Nodes (3): Disabled, PlaidProvider, Provider

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 55 - "harness"
Cohesion: 0.26
Nodes (9): net/http/httptest.Server, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders() (+1 more)

### Community 57 - "NewRenderer"
Cohesion: 0.33
Nodes (5): Load(), parseAcceptLanguage(), parseQ(), TestTemplateKeysResolve(), NewRenderer()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 62 - ".serveAsset"
Cohesion: 0.50
Nodes (3): net/http.HandlerFunc, Asset(), TestCovAsset()

### Community 63 - "TestCov3BankConvertRedirect"
Cohesion: 0.83
Nodes (3): covSeedBankTx(), TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 64 - "seedDirectExpense"
Cohesion: 0.20
Nodes (19): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), formValues() (+11 more)

### Community 66 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

## Knowledge Gaps
- **95 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+90 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **45 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `net/http.Request`, `testing.T`?**
  _High betweenness centrality (0.058) - this node is a cross-community bridge._
- **Why does `Open()` connect `NewManager` to `Config`, `body`, `openTestStore`, `newTestService`, `context.Context`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._
- **Why does `User` connect `context.Context` to `handlers_pages.go`, `covUser`, `net/http.Request`, `NewManager`, `openTestStore`, `Transaction`, `Service`, `RandomToken`, `Expense`, `Renderer`?**
  _High betweenness centrality (0.036) - this node is a cross-community bridge._
- **Are the 90 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 90 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _95 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.Request` be split into smaller, more focused modules?**
  _Cohesion score 0.057710249233639876 - nodes in this community are weakly interconnected._
- **Should `NewManager` be split into smaller, more focused modules?**
  _Cohesion score 0.06935908691834942 - nodes in this community are weakly interconnected._