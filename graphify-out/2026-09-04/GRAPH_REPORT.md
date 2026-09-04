# Graph Report - gosplit  (2026-09-01)

## Corpus Check
- 153 files · ~120,628 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1471 nodes · 4048 edges · 126 communities (80 shown, 46 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 733 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `974fdff0`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- .computeGroupSettlements
- newTestService
- net/http.Request
- NewManager
- Base Layout
- Load
- atoi64
- body
- openTestStore
- Method
- ExpenseRecurrence
- app service
- Group
- Format
- Group Detail Page
- service_test.go
- nowISO
- Store
- database/sql query layer (as-built)
- dev.sh
- RandomToken
- Expense
- currency/zz_coverage_test.go
- nullInt
- helpers.go
- context.Context
- convert.js
- collapseView
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- ctxTimeout
- Auth (magic-link + password)
- SQLite default (no DB container)
- idiomorph-ext.min.js
- Currency conversion (pluggable providers)
- dev.ps1
- mail_test.go
- Service
- Transaction
- expense_form.js
- service/zz_coverage2_test.go
- tripHarness
- Store
- web/zz_coverage_test.go
- Store
- Convert
- mkExp
- PlaidProvider
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- covUser
- .CreateConversionExact
- harness
- handlers_recurring.go
- provider_test.go
- bank.js
- GoSplit App Icon
- bankTxRow
- .handleGroupDetail
- Renderer
- TestCov3BankConvertRedirect
- testing.T
- sw.js
- New
- htmx.min.js
- github.com/hafio/gosplit
- Service
- LineFromInput
- GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)
- Bundle
- TestThemeColorUpdate
- .buildBatches
- New
- ValidTheme
- run
- admin_test.go
- load
- NewRenderer
- nameCache
- TestActivityFeedShowAll
- .serveAsset
- TestExpenseFormRendersComponents
- TestArchivedGroupHiddenFromActivityAndSection
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
1. `body()` - 118 edges
2. `newHarness()` - 88 edges
3. `newTestService()` - 73 edges
4. `ctxTimeout()` - 63 edges
5. `Expense` - 47 edges
6. `covUser()` - 45 edges
7. `User` - 43 edges
8. `openTestStore()` - 39 edges
9. `atoi64()` - 32 edges
10. `Server` - 28 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `NewManager()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/auth/middleware.go
- `run()` --calls--> `Load()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/config/config.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/httpapp/server.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/mail/mail.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/gosplit/main.go → internal/scheduler/scheduler.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (126 total, 46 thin omitted)

### Community 0 - ".computeGroupSettlements"
Cohesion: 0.20
Nodes (12): Transfer, balanceRow, currencyNet, groupRow, settlementRow, Simplify(), netAfter(), TestSimplify_GoldenUSDTransactionCount() (+4 more)

### Community 1 - "newTestService"
Cohesion: 0.13
Nodes (21): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestCreateConversionExact(), TestAddFriendToGroup(), Service, newTestService(), generatedFrom(), Service (+13 more)

### Community 2 - "net/http.Request"
Cohesion: 0.10
Nodes (9): net/http.Request, net/http.ResponseWriter, Server, safeNext(), Server, Server, Server, Server (+1 more)

### Community 3 - "NewManager"
Cohesion: 0.15
Nodes (29): ctxKey, net/http.Handler, CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler() (+21 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.17
Nodes (19): getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList(), Load(), looksLikePostgresKeywordDSN(), TestGetEnv() (+11 more)

### Community 6 - "atoi64"
Cohesion: 0.12
Nodes (12): Server, applySubmittedFields(), displayName(), fieldErrorf(), fieldErrorsFor(), formVersion(), Server, parseExpenseInput() (+4 more)

### Community 7 - "body"
Cohesion: 0.08
Nodes (57): TestExpenseNoteAndCollapse(), TestConvertExactBothAmounts(), TestRateEndpoint(), TestBoostedRequestStillGetsFullPage(), TestFlashNotInRedirectURL(), TestFlashShownOnceThenGone(), TestFragmentBranchOmitsLayout(), TestPlainRequestIgnoresStrayTargetHeader() (+49 more)

### Community 8 - "openTestStore"
Cohesion: 0.07
Nodes (52): TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit(), TestGroupMemberCountsAndNets(), TestListFriendExpensesFilters(), TestAdminUpdateUser(), TestListFriendExpensesIncludesGroups(), buildAmounts() (+44 more)

### Community 9 - "Method"
Cohesion: 0.08
Nodes (49): candidate, expenseDetailView, fieldError, participantView, formatValue(), lineForMethod(), orZero(), prefillDerived() (+41 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.15
Nodes (7): time.Time, Service, toISO(), todayISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "app service"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.24
Nodes (3): Store, scanGroup(), Group

### Community 13 - "Format"
Cohesion: 0.16
Nodes (13): absInt64(), Server, friendSettlePrefill(), groupSettleSuggestion(), decimals(), Format(), Parse(), pow10() (+5 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "service_test.go"
Cohesion: 0.16
Nodes (11): extractToken(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion(), TestForgotResetPassword(), TestImportFromSplitwise(), TestMagicLinkFlow(), TestRegisterAndLogin() (+3 more)

### Community 16 - "nowISO"
Cohesion: 0.11
Nodes (12): time.Duration, Store, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store (+4 more)

### Community 17 - "Store"
Cohesion: 0.22
Nodes (6): database/sql.NullInt64, friendRow, CumulatedBalance, Store, scanBalances(), Balance

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 21 - "Expense"
Cohesion: 0.14
Nodes (12): database/sql.Tx, FormatWithCode(), Service, BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, inPlaceholders() (+4 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.11
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "nullInt"
Cohesion: 0.24
Nodes (8): TestDeleteExpenseAuthorization(), TestDeleteExpenseRejectsStaleVersion(), nullInt(), TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted(), TestMoveExpenseUnauthorized()

### Community 24 - "helpers.go"
Cohesion: 0.16
Nodes (14): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), methodGlyph(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange() (+6 more)

### Community 25 - "context.Context"
Cohesion: 0.09
Nodes (13): context.Context, Service, Service, swGet(), Service, Store, Store, User (+5 more)

### Community 26 - "convert.js"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 32 - "ctxTimeout"
Cohesion: 0.16
Nodes (4): context.CancelFunc, Server, sortedNets(), ctxTimeout()

### Community 34 - "SQLite default (no DB container)"
Cohesion: 0.67
Nodes (3): Postgres db service (optional, commented), SQLite default (no DB container), SQLite (pure-Go modernc.org/sqlite)

### Community 35 - "idiomorph-ext.min.js"
Cohesion: 0.31
Nodes (15): a(), c(), d(), e(), f(), h(), i(), l() (+7 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "mail_test.go"
Cohesion: 0.13
Nodes (15): crypto/tls.Config, net.Listener, net/smtp.Client, buildMessage(), Mailer, New(), serveFakeSMTP(), TestBuildMessage() (+7 more)

### Community 39 - "Service"
Cohesion: 0.24
Nodes (8): database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullStr(), orDefault(), sameGroup()

### Community 40 - "Transaction"
Cohesion: 0.12
Nodes (6): Disabled, Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 42 - "service/zz_coverage2_test.go"
Cohesion: 0.18
Nodes (9): covMultipartReq(), TestCovBankConnectedFlow(), TestCovBankProviderErrors(), TestCovConversionErrors(), TestCovEmailParticipants(), TestCovInviteSendsPendingInvites(), TestCovPushGating(), TestCovUpdateAvatar() (+1 more)

### Community 43 - "tripHarness"
Cohesion: 0.14
Nodes (29): settlementPath(), TestEditSettlementGroupMoveNeedsAck(), TestEditSettlementHidesFixedFieldPickers(), TestEditSettlementKeepsZeroDecimalCurrency(), TestEditSettlementNonMemberRendersFieldError(), TestEditSettlementPageHasNoMethodPicker(), TestEditSettlementRejectsBadAmount(), TestEditSettlementRejectsCurrencySwitch() (+21 more)

### Community 45 - "web/zz_coverage_test.go"
Cohesion: 0.21
Nodes (13): currencyCodes(), csvRows(), covRenderer(), TestCovAssetsHandlerCacheControl(), TestCovCSVRows(), TestCovCurrencyCodes(), TestCovFuncsAbs64(), TestCovFuncsDict() (+5 more)

### Community 46 - "Store"
Cohesion: 0.20
Nodes (12): database/sql.DB, Config, Engine, New(), prefixCols(), trimSpace(), Store, Open() (+4 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.44
Nodes (9): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+1 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - "covUser"
Cohesion: 0.20
Nodes (17): covUser(), Service, TestAddExpenseDirectCreatesFriendship(), TestAddExpenseNonMemberRejected(), TestAddFriendByEmail(), TestAdminMagicLinkForUser(), TestArchiveDirectRejectsSelf(), TestArchiveGroupNonMember() (+9 more)

### Community 55 - "harness"
Cohesion: 0.19
Nodes (10): net/http/httptest.Server, net/http.Response, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect() (+2 more)

### Community 57 - "provider_test.go"
Cohesion: 0.25
Nodes (13): roundTripFunc, Provider, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken() (+5 more)

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - ".handleGroupDetail"
Cohesion: 0.21
Nodes (14): expenseRow, monthGroup, TestFlashCookieRoundTrip(), TestTakeFlashClearsCookie(), applyFeedLimit(), dateOnly(), groupByMonth(), monthLabel() (+6 more)

### Community 62 - "Renderer"
Cohesion: 0.19
Nodes (5): html/template.Template, TestQueryWithout(), Renderer, ViewData, queryWithout()

### Community 63 - "TestCov3BankConvertRedirect"
Cohesion: 0.83
Nodes (3): covSeedBankTx(), TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 64 - "testing.T"
Cohesion: 0.23
Nodes (23): testing.T, seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord() (+15 more)

### Community 66 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

### Community 82 - "Service"
Cohesion: 0.26
Nodes (8): New(), newSchedService(), TestNew(), TestRunStopsOnCancel(), TestTickAsLeader(), TestTickNonLeader(), Service, Scheduler

### Community 83 - "LineFromInput"
Cohesion: 0.29
Nodes (10): TestAddExpenseRecordsTypedInputs(), DecodeInputs(), EncodeInputs(), Line, LineFromInput(), TestDecodeInputsRejects(), TestEncodeDecodeInputs(), TestEncodeInputsSkipsSystemMethods() (+2 more)

### Community 84 - "GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)"
Cohesion: 0.18
Nodes (10): Context, Deviations from the approved plan, Docs (same change), Explicitly deferred / rejected, GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural), Ledger, Tests (same change as the code they cover), Tier 1 -- quick wins (independent, ship in any order) (+2 more)

### Community 85 - "Bundle"
Cohesion: 0.25
Nodes (5): Bundle, Load(), parseAcceptLanguage(), parseQ(), TestTemplateKeysResolve()

### Community 87 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 88 - "New"
Cohesion: 0.18
Nodes (9): New(), TestCovAuthErrorBranches(), TestCovDispatchDefaults(), TestCovEmailParticipantsSendError(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovPushEnabledSendError(), TestCovRecurrenceErrorBranches() (+1 more)

### Community 89 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 90 - "run"
Cohesion: 0.31
Nodes (7): main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion(), wantsVersion(), log/slog.Level

### Community 91 - "admin_test.go"
Cohesion: 0.24
Nodes (5): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestFilterChipsRender()

### Community 92 - "load"
Cohesion: 0.52
Nodes (6): firstOther(), load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate()

### Community 93 - "NewRenderer"
Cohesion: 0.62
Nodes (6): feedViewData(), TestFragmentMatchesRegionInFullPage(), TestRenderFragmentOmitsLayout(), TestRenderFragmentSetsNoStoreHeaders(), TestRenderFragmentUnknownFailsLoudly(), NewRenderer()

### Community 95 - "TestActivityFeedShowAll"
Cohesion: 0.40
Nodes (4): TestActivityFeedShowAll(), TestFeedDateParts(), TestGroupByMonth(), feedDateParts()

### Community 96 - ".serveAsset"
Cohesion: 0.50
Nodes (3): net/http.HandlerFunc, Asset(), TestCovAsset()

### Community 97 - "TestExpenseFormRendersComponents"
Cohesion: 0.50
Nodes (3): TestExpenseFormRendersComponents(), TestExpenseFormRoundTripsMethods(), TestExpenseFormTargetSelector()

## Knowledge Gaps
- **104 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+99 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **46 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Open()` connect `Store` to `newTestService`, `NewManager`, `body`, `openTestStore`, `Service`, `context.Context`, `run`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `testing.T`, `net/http.Request`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `Expense` connect `Expense` to `newTestService`, `atoi64`, `Service`, `openTestStore`, `Method`, `nowISO`, `Store`, `.CreateConversionExact`, `.buildBatches`, `.handleGroupDetail`, `nameCache`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._
- **Are the 108 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 108 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _104 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestService` be split into smaller, more focused modules?**
  _Cohesion score 0.13 - nodes in this community are weakly interconnected._
- **Should `net/http.Request` be split into smaller, more focused modules?**
  _Cohesion score 0.10372340425531915 - nodes in this community are weakly interconnected._