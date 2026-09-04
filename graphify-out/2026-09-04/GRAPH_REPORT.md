# Graph Report - gosplit  (2026-09-04)

## Corpus Check
- 163 files · ~137,602 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1621 nodes · 4453 edges · 124 communities (81 shown, 43 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 773 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4462e4ee`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- handlers_expenses.go
- covUser
- net/http.ResponseWriter
- NewManager
- Base Layout
- Load
- Server
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
- nameCache
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
- net/http.Request
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
- testing.T
- Store
- Convert
- mkExp
- index.go
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- newTestService
- .CreateConversionExact
- harness
- handlers_recurring.go
- PlaidProvider
- bank.js
- GoSplit App Icon
- bankTxRow
- handlers_helpers.go
- Renderer
- TestCov3BankConvertRedirect
- seedDirectExpense
- sw.js
- New
- htmx.min.js
- github.com/hafio/gosplit
- newSchedService
- crypto.go
- GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)
- Validated
- TestThemeColorUpdate
- .buildBatches
- New
- Table
- run
- Server
- Header
- .Dump
- backup_cli.go
- encoding/json.RawMessage
- CheckAutoRestore
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
- `loadCLIConfig()` --references--> `Config`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `loadCLIConfig()` --calls--> `Load()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `runBackupCLI()` --calls--> `OpenNoMigrate()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/store/store.go
- `runRestoreCLI()` --calls--> `OpenNoMigrate()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/store/store.go
- `runInspectCLI()` --calls--> `InspectFile()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/backup/restore.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (124 total, 43 thin omitted)

### Community 0 - "handlers_expenses.go"
Cohesion: 0.12
Nodes (27): candidate, fieldError, applySubmittedFields(), fieldErrorf(), formatValue(), formVersion(), orZero(), parseSettlementInput() (+19 more)

### Community 1 - "covUser"
Cohesion: 0.19
Nodes (19): generatedFrom(), Service, TestAddExpenseOmitsNonSplittingPayer(), TestAddExpenseRecordsTypedInputs(), TestGenerateOneCopiesWhenNoInputs(), TestGenerateOneReSplitsFromInputs(), TestMoveExpenseRejectsSettlement(), TestMoveExpenseRejectsStaleVersion() (+11 more)

### Community 2 - "net/http.ResponseWriter"
Cohesion: 0.10
Nodes (6): net/http.ResponseWriter, Server, safeNext(), Server, Server, Server

### Community 3 - "NewManager"
Cohesion: 0.15
Nodes (29): ctxKey, net/http.Handler, CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler() (+21 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.14
Nodes (22): cron.Schedule, BackupSchedule(), getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList(), Load() (+14 more)

### Community 6 - "Server"
Cohesion: 0.16
Nodes (6): displayName(), fieldErrorsFor(), friendSettlePrefill(), Server, groupSettleSuggestion(), parseExpenseInput()

### Community 7 - "body"
Cohesion: 0.08
Nodes (55): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestConvertExactBothAmounts(), TestRateEndpoint() (+47 more)

### Community 8 - "openTestStore"
Cohesion: 0.06
Nodes (57): Transfer, Simplify(), netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit() (+49 more)

### Community 9 - "Method"
Cohesion: 0.11
Nodes (35): lineForMethod(), todayISO(), EncodeInputs(), Line, LineFromInput(), TestDecodeInputsRejects(), TestEncodeDecodeInputs(), TestEncodeInputsSkipsSystemMethods() (+27 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.18
Nodes (5): Service, toISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "app service"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.17
Nodes (7): inPlaceholders(), prefixCols(), trimSpace(), Store, scanGroup(), Group, TestCov2PureHelpers()

### Community 13 - "Format"
Cohesion: 0.19
Nodes (11): absInt64(), Server, decimals(), Format(), Parse(), pow10(), TestFormat(), TestFormatWithCode() (+3 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "service_test.go"
Cohesion: 0.16
Nodes (11): extractToken(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion(), TestForgotResetPassword(), TestImportFromSplitwise(), TestMagicLinkFlow(), TestRegisterAndLogin() (+3 more)

### Community 16 - "nowISO"
Cohesion: 0.12
Nodes (11): time.Duration, Store, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store (+3 more)

### Community 17 - "nameCache"
Cohesion: 0.12
Nodes (14): balanceRow, currencyNet, friendRow, groupRow, nameCache, settlementRow, Server, max64() (+6 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 21 - "Expense"
Cohesion: 0.14
Nodes (11): FormatWithCode(), Service, BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, scanExpense(), Expense (+3 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.11
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "nullInt"
Cohesion: 0.24
Nodes (8): TestDeleteExpenseAuthorization(), TestDeleteExpenseRejectsStaleVersion(), nullInt(), TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted(), TestMoveExpenseUnauthorized()

### Community 24 - "helpers.go"
Cohesion: 0.21
Nodes (11): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+3 more)

### Community 25 - "context.Context"
Cohesion: 0.09
Nodes (13): context.Context, Service, Service, swGet(), Service, Store, Store, User (+5 more)

### Community 26 - "convert.js"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 32 - "net/http.Request"
Cohesion: 0.14
Nodes (12): context.CancelFunc, net/http.Request, Server, applyFeedLimit(), parseFilter(), showAllHref(), Server, sortedNets() (+4 more)

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
Cohesion: 0.22
Nodes (9): database/sql.NullInt64, database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullStr(), orDefault() (+1 more)

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

### Community 45 - "testing.T"
Cohesion: 0.09
Nodes (33): TestVersionDefault(), TestWantsVersion(), testing.T, TestExpenseFormRendersComponents(), TestExpenseFormRoundTripsMethods(), TestExpenseFormTargetSelector(), TestActivityFeedShowAll(), TestFeedDateParts() (+25 more)

### Community 46 - "Store"
Cohesion: 0.17
Nodes (14): database/sql.DB, Config, Engine, New(), Service, connect(), Store, Open() (+6 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.44
Nodes (9): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+1 more)

### Community 49 - "index.go"
Cohesion: 0.21
Nodes (20): countingReader, Entry, EntryKind, Index, Limits, Opener, archive/tar.Reader, io.Reader (+12 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - "newTestService"
Cohesion: 0.13
Nodes (21): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestCreateConversionExact(), TestAddFriendToGroup(), Service, newTestService(), TestAddExpenseDirectCreatesFriendship(), TestAddExpenseNonMemberRejected() (+13 more)

### Community 55 - "harness"
Cohesion: 0.19
Nodes (10): net/http/httptest.Server, net/http.Response, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect() (+2 more)

### Community 57 - "PlaidProvider"
Cohesion: 0.16
Nodes (14): PlaidProvider, roundTripFunc, Provider, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken() (+6 more)

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - "handlers_helpers.go"
Cohesion: 0.31
Nodes (7): expenseDetailView, expenseRow, monthGroup, participantView, dateOnly(), groupByMonth(), monthLabel()

### Community 62 - "Renderer"
Cohesion: 0.06
Nodes (34): html/template.Template, net/http.HandlerFunc, Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load() (+26 more)

### Community 63 - "TestCov3BankConvertRedirect"
Cohesion: 0.83
Nodes (3): covSeedBankTx(), TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 64 - "seedDirectExpense"
Cohesion: 0.17
Nodes (22): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+14 more)

### Community 66 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

### Community 82 - "newSchedService"
Cohesion: 0.35
Nodes (7): New(), newSchedService(), TestNew(), TestRunStopsOnCancel(), TestTickAsLeader(), TestTickNonLeader(), Scheduler

### Community 83 - "crypto.go"
Cohesion: 0.16
Nodes (14): Keyring, openReader, sealWriter, crypto/cipher.AEAD, io.WriteCloser, expand(), newAEAD(), NewKeyring() (+6 more)

### Community 84 - "GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)"
Cohesion: 0.18
Nodes (10): Context, Deviations from the approved plan, Docs (same change), Explicitly deferred / rejected, GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural), Ledger, Tests (same change as the code they cover), Tier 1 -- quick wins (independent, ship in any order) (+2 more)

### Community 85 - "Validated"
Cohesion: 0.20
Nodes (8): ApplyOptions, Report, Validated, dirExists(), RecoverIncompleteSwap(), Runner, oldDirFor(), stagingDirFor()

### Community 87 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 88 - "New"
Cohesion: 0.18
Nodes (9): New(), TestCovAuthErrorBranches(), TestCovDispatchDefaults(), TestCovEmailParticipantsSendError(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovPushEnabledSendError(), TestCovRecurrenceErrorBranches() (+1 more)

### Community 89 - "Table"
Cohesion: 0.22
Nodes (8): batcher, Table, database/sql.Tx, DeleteOrder(), InsertOrder(), LookupTable(), quoteIdent(), quoteIdents()

### Community 90 - "run"
Cohesion: 0.43
Nodes (6): wantsBackup(), main(), parseLogLevel(), run(), wantsVersion(), log/slog.Level

### Community 91 - "Server"
Cohesion: 0.19
Nodes (6): TestFlashCookieRoundTrip(), TestTakeFlashClearsCookie(), Server, navSlug(), setFlash(), takeFlash()

### Community 92 - "Header"
Cohesion: 0.24
Nodes (9): Header, TableStat, io.Writer, decodeB64(), Manifest, ReadHeader(), tablePartName(), WriteHeader() (+1 more)

### Community 93 - ".Dump"
Cohesion: 0.30
Nodes (8): queryer, archive/tar.Writer, time.Time, b64(), Runner, readMigrationVersions(), SameMigrationSet(), writeTarFile()

### Community 94 - "backup_cli.go"
Cohesion: 0.38
Nodes (11): cliContext(), humanBytes(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI() (+3 more)

### Community 95 - "encoding/json.RawMessage"
Cohesion: 0.36
Nodes (10): Column, ColumnKind, encoding/json.RawMessage, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), ScanDest() (+2 more)

### Community 96 - "CheckAutoRestore"
Cohesion: 0.52
Nodes (6): acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), orDashStr(), probeWritable(), writeMarker()

## Knowledge Gaps
- **104 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+99 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **43 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Open()` connect `Store` to `NewManager`, `body`, `openTestStore`, `newSchedService`, `newTestService`, `context.Context`, `run`?**
  _High betweenness centrality (0.048) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `net/http.Request` to `handlers_expenses.go`, `net/http.ResponseWriter`, `Server`, `Format`, `context.Context`, `handlers_helpers.go`?**
  _High betweenness centrality (0.035) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `net/http.Request`, `testing.T`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Are the 108 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 108 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _104 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `handlers_expenses.go` be split into smaller, more focused modules?**
  _Cohesion score 0.12043010752688173 - nodes in this community are weakly interconnected._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.09879032258064516 - nodes in this community are weakly interconnected._