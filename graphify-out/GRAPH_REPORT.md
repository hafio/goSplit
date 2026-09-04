# Graph Report - gosplit  (2026-09-04)

## Corpus Check
- 169 files · ~149,182 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1726 nodes · 4905 edges · 130 communities (88 shown, 42 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 914 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `683bd09a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Method
- LineFromInput
- ctxTimeout
- NewManager
- Base Layout
- Load
- net/http.Request
- body
- openTestStore
- Compute
- ExpenseRecurrence
- app service
- Group
- Format
- Group Detail Page
- testKeyring
- nowISO
- nameCache
- database/sql query layer (as-built)
- dev.sh
- RandomToken
- Expense
- currency/zz_coverage_test.go
- newTestRunner
- helpers.go
- context.Context
- convert.js
- collapseView
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- net/http.ResponseWriter
- Auth (magic-link + password)
- SQLite default (no DB container)
- idiomorph-ext.min.js
- Currency conversion (pluggable providers)
- dev.ps1
- mail_test.go
- Service
- Transaction
- expense_form.js
- index_test.go
- tripHarness
- User
- web/zz_coverage_test.go
- Store
- Convert
- mkExp
- Scan
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- testing.T
- atoi64
- harness
- handlers_recurring.go
- PlaidProvider
- bank.js
- GoSplit App Icon
- bankTxRow
- .handleGroupDetail
- Renderer
- Bundle
- seedDirectExpense
- sw.js
- New
- htmx.min.js
- github.com/hafio/gosplit
- Service
- crypto.go
- GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)
- Validated
- TestThemeColorUpdate
- .buildBatches
- ValidTheme
- InsertOrder
- run
- Server
- Header
- .Dump
- backup_cli.go
- codec_test.go
- CheckAutoRestore
- Simplify
- load
- golden_test.go
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
- NewRenderer
- .ImportFromSplitwise
- .serveAsset

## God Nodes (most connected - your core abstractions)
1. `body()` - 118 edges
2. `newHarness()` - 88 edges
3. `newTestService()` - 73 edges
4. `ctxTimeout()` - 63 edges
5. `Expense` - 47 edges
6. `covUser()` - 45 edges
7. `User` - 43 edges
8. `openTestStore()` - 39 edges
9. `testKeyring()` - 35 edges
10. `Store` - 35 edges

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

## Communities (130 total, 42 thin omitted)

### Community 0 - "Method"
Cohesion: 0.15
Nodes (25): candidate, expenseDetailView, fieldError, participantView, formatValue(), lineForMethod(), orZero(), prefillDerived() (+17 more)

### Community 1 - "LineFromInput"
Cohesion: 0.26
Nodes (11): DecodeInputs(), EncodeInputs(), Line, LineFromInput(), TestDecodeInputsRejects(), TestEncodeDecodeInputs(), TestEncodeInputsSkipsSystemMethods(), TestLineInputEqualAndUnknown() (+3 more)

### Community 2 - "ctxTimeout"
Cohesion: 0.09
Nodes (7): context.CancelFunc, Server, safeNext(), Server, Server, Server, ctxTimeout()

### Community 3 - "NewManager"
Cohesion: 0.14
Nodes (29): ctxKey, net/http.Handler, CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler() (+21 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.14
Nodes (22): cron.Schedule, BackupSchedule(), getEnv(), getEnvBool(), getEnvDuration(), getEnvInt(), getEnvList(), Load() (+14 more)

### Community 6 - "net/http.Request"
Cohesion: 0.16
Nodes (11): net/http.Request, applySubmittedFields(), displayName(), fieldErrorf(), fieldErrorsFor(), formVersion(), Server, parseExpenseInput() (+3 more)

### Community 7 - "body"
Cohesion: 0.07
Nodes (67): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestConvertExactBothAmounts(), TestRateEndpoint() (+59 more)

### Community 8 - "openTestStore"
Cohesion: 0.08
Nodes (46): TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit(), TestGroupMemberCountsAndNets(), TestListFriendExpensesFilters(), TestAdminUpdateUser(), TestListFriendExpensesIncludesGroups(), Store (+38 more)

### Community 9 - "Compute"
Cohesion: 0.19
Nodes (22): TestSplitRemainderStableAcrossLineOrder(), Compute(), computeShares(), DefaultSplitAllowed(), distributeRemainder(), finalize(), gcd(), Line (+14 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.17
Nodes (6): Service, toISO(), todayISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "app service"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.20
Nodes (5): database/sql.NullString, nullStr(), Store, scanGroup(), Group

### Community 13 - "Format"
Cohesion: 0.18
Nodes (11): absInt64(), Server, decimals(), Format(), Parse(), pow10(), TestFormat(), TestFormatWithCode() (+3 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "testKeyring"
Cohesion: 0.16
Nodes (28): Keyring, io.WriteCloser, newAEAD(), NewOpenReader(), NewSealWriter(), errString(), frameAt(), open() (+20 more)

### Community 16 - "nowISO"
Cohesion: 0.09
Nodes (13): time.Duration, Service, Service, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store (+5 more)

### Community 17 - "nameCache"
Cohesion: 0.11
Nodes (16): balanceRow, currencyNet, friendRow, groupRow, nameCache, settlementRow, friendSettlePrefill(), groupSettleSuggestion() (+8 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 21 - "Expense"
Cohesion: 0.15
Nodes (11): FormatWithCode(), Service, BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, inPlaceholders(), scanExpense() (+3 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.07
Nodes (40): roundTripFunc, covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, net/http.Response, New() (+32 more)

### Community 23 - "newTestRunner"
Cohesion: 0.22
Nodes (25): countRows(), exec(), newTestRunner(), newTestRunnerIn(), seedEverything(), seedUploads(), snapshotTables(), wipeAll() (+17 more)

### Community 24 - "helpers.go"
Cohesion: 0.17
Nodes (13): avatarColor(), categories(), categoryEmoji(), currencyCodes(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange() (+5 more)

### Community 25 - "context.Context"
Cohesion: 0.07
Nodes (11): context.Context, Store, Store, Store, Store, Store, Store, covErrRates (+3 more)

### Community 26 - "convert.js"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 32 - "net/http.ResponseWriter"
Cohesion: 0.16
Nodes (4): net/http.ResponseWriter, Server, Server, sortedNets()

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
Cohesion: 0.23
Nodes (8): database/sql.NullInt64, ExpenseInput, Service, SettlementInput, movable(), nullInt(), orDefault(), sameGroup()

### Community 40 - "Transaction"
Cohesion: 0.18
Nodes (5): Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 42 - "index_test.go"
Cohesion: 0.31
Nodes (24): craftEntry, ReadHeader(), tablePartName(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive(), itoa() (+16 more)

### Community 43 - "tripHarness"
Cohesion: 0.14
Nodes (29): settlementPath(), TestEditSettlementGroupMoveNeedsAck(), TestEditSettlementHidesFixedFieldPickers(), TestEditSettlementKeepsZeroDecimalCurrency(), TestEditSettlementNonMemberRendersFieldError(), TestEditSettlementPageHasNoMethodPicker(), TestEditSettlementRejectsBadAmount(), TestEditSettlementRejectsCurrencySwitch() (+21 more)

### Community 44 - "User"
Cohesion: 0.15
Nodes (7): Service, prefixCols(), trimSpace(), User, orDefault(), scanUser(), TestCov2PureHelpers()

### Community 45 - "web/zz_coverage_test.go"
Cohesion: 0.21
Nodes (13): methodGlyph(), csvRows(), covRenderer(), TestCovAssetsHandlerCacheControl(), TestCovCSVRows(), TestCovFuncsAbs64(), TestCovFuncsDict(), TestCovMethodGlyph() (+5 more)

### Community 46 - "Store"
Cohesion: 0.20
Nodes (12): database/sql.DB, Config, Engine, connect(), Store, Open(), OpenNoMigrate(), sqliteDir() (+4 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.53
Nodes (8): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency()

### Community 49 - "Scan"
Cohesion: 0.17
Nodes (23): countingReader, Entry, EntryKind, Index, Limits, Opener, archive/tar.Reader, io.Reader (+15 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - "testing.T"
Cohesion: 0.07
Nodes (73): testing.T, TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestArchiveNothing(), TestCreateConversionExact(), TestDeleteExpenseAuthorization(), TestDeleteExpenseRejectsStaleVersion(), TestAddFriendToGroup() (+65 more)

### Community 55 - "harness"
Cohesion: 0.19
Nodes (12): net/http/httptest.Server, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders() (+4 more)

### Community 57 - "PlaidProvider"
Cohesion: 0.17
Nodes (3): Disabled, PlaidProvider, Provider

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - ".handleGroupDetail"
Cohesion: 0.24
Nodes (12): expenseRow, monthGroup, TestFeedDateParts(), TestGroupByMonth(), applyFeedLimit(), dateOnly(), feedDateParts(), groupByMonth() (+4 more)

### Community 62 - "Renderer"
Cohesion: 0.19
Nodes (6): html/template.Template, TestQueryWithout(), assetURL(), Renderer, ViewData, queryWithout()

### Community 63 - "Bundle"
Cohesion: 0.25
Nodes (5): Bundle, Load(), parseAcceptLanguage(), parseQ(), TestTemplateKeysResolve()

### Community 64 - "seedDirectExpense"
Cohesion: 0.17
Nodes (22): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+14 more)

### Community 66 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

### Community 82 - "Service"
Cohesion: 0.21
Nodes (10): New(), New(), newSchedService(), TestNew(), TestRunStopsOnCancel(), TestTickAsLeader(), TestTickNonLeader(), Service (+2 more)

### Community 83 - "crypto.go"
Cohesion: 0.18
Nodes (11): openReader, sealWriter, crypto/cipher.AEAD, expand(), NewKeyring(), NewNoncePrefix(), NewSalt(), nonceFor() (+3 more)

### Community 84 - "GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)"
Cohesion: 0.18
Nodes (10): Context, Deviations from the approved plan, Docs (same change), Explicitly deferred / rejected, GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural), Ledger, Tests (same change as the code they cover), Tier 1 -- quick wins (independent, ship in any order) (+2 more)

### Community 85 - "Validated"
Cohesion: 0.19
Nodes (8): ApplyOptions, batcher, Report, Validated, database/sql.Tx, Runner, InspectFile(), oldDirFor()

### Community 87 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 88 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 89 - "InsertOrder"
Cohesion: 0.16
Nodes (18): Column, ColumnKind, Table, DeleteOrder(), InsertOrder(), isBareIdent(), TableCount(), TestDeleteOrderIsExactReverse() (+10 more)

### Community 90 - "run"
Cohesion: 0.21
Nodes (10): wantsBackup(), main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion(), wantsVersion(), log/slog.Level (+2 more)

### Community 91 - "Server"
Cohesion: 0.20
Nodes (6): TestFlashCookieRoundTrip(), TestTakeFlashClearsCookie(), Server, navSlug(), setFlash(), takeFlash()

### Community 92 - "Header"
Cohesion: 0.29
Nodes (6): Header, TableStat, io.Writer, decodeB64(), Manifest, WriteHeader()

### Community 93 - ".Dump"
Cohesion: 0.24
Nodes (10): queryer, archive/tar.Writer, time.Time, b64(), Runner, readMigrationVersions(), SameMigrationSet(), writeTarFile() (+2 more)

### Community 94 - "backup_cli.go"
Cohesion: 0.38
Nodes (11): cliContext(), humanBytes(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI() (+3 more)

### Community 95 - "codec_test.go"
Cohesion: 0.17
Nodes (23): database/sql.NullBool, encoding/json.RawMessage, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), ScanDest(), nullBool() (+15 more)

### Community 96 - "CheckAutoRestore"
Cohesion: 0.52
Nodes (6): acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), orDashStr(), probeWritable(), writeMarker()

### Community 97 - "Simplify"
Cohesion: 0.48
Nodes (5): Transfer, Simplify(), netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles()

### Community 98 - "load"
Cohesion: 0.52
Nodes (6): firstOther(), load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate()

### Community 99 - "golden_test.go"
Cohesion: 0.57
Nodes (6): buildAmounts(), generateGolden(), mod(), sharesFor(), TestGoldenScenario(), goldenTxn

### Community 127 - "NewRenderer"
Cohesion: 0.62
Nodes (6): feedViewData(), TestFragmentMatchesRegionInFullPage(), TestRenderFragmentOmitsLayout(), TestRenderFragmentSetsNoStoreHeaders(), TestRenderFragmentUnknownFailsLoudly(), NewRenderer()

### Community 128 - ".ImportFromSplitwise"
Cohesion: 0.47
Nodes (3): Service, swGet(), ImportResult

### Community 129 - ".serveAsset"
Cohesion: 0.50
Nodes (3): net/http.HandlerFunc, Asset(), TestCovAsset()

## Knowledge Gaps
- **104 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+99 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `covMultipartReq()` connect `testing.T` to `net/http.Request`?**
  _High betweenness centrality (0.046) - this node is a cross-community bridge._
- **Why does `Open()` connect `Store` to `NewManager`, `body`, `openTestStore`, `Service`, `testing.T`, `newTestRunner`, `context.Context`, `run`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `ctxTimeout` to `net/http.ResponseWriter`, `net/http.Request`, `Format`, `atoi64`, `context.Context`, `Server`, `.handleGroupDetail`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **Are the 108 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 108 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _104 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Method` be split into smaller, more focused modules?**
  _Cohesion score 0.1455026455026455 - nodes in this community are weakly interconnected._
- **Should `ctxTimeout` be split into smaller, more focused modules?**
  _Cohesion score 0.09425287356321839 - nodes in this community are weakly interconnected._