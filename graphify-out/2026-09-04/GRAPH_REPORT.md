# Graph Report - gosplit  (2026-09-04)

## Corpus Check
- 181 files · ~173,182 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1903 nodes · 5349 edges · 133 communities (90 shown, 43 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 985 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `af25c245`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Method
- covUser
- net/http.ResponseWriter
- NewManager
- Base Layout
- Load
- Server
- body
- openTestStore
- Implementation
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
- service_test.go
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
- index_test.go
- tripHarness
- Store
- testing.T
- Store
- Convert
- mkExp
- Scan
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- newTestService
- service/zz_coverage2_test.go
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
- JobTracker
- Keyring
- GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)
- RecoverIncompleteSwap
- TestThemeColorUpdate
- .buildBatches
- ValidTheme
- InsertOrder
- run
- net/http.Request
- Header
- .Dump
- New
- codec_test.go
- CheckAutoRestore
- New
- load
- nullInt
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
- Store
- .serveAsset
- TestActivityFeedShowAll
- TestCov3BankConvertRedirect
- TestQueryWithout

## God Nodes (most connected - your core abstractions)
1. `body()` - 123 edges
2. `newHarness()` - 93 edges
3. `newTestService()` - 73 edges
4. `ctxTimeout()` - 64 edges
5. `Expense` - 47 edges
6. `covUser()` - 45 edges
7. `User` - 43 edges
8. `openTestStore()` - 41 edges
9. `testKeyring()` - 35 edges
10. `Store` - 35 edges

## Surprising Connections (you probably didn't know these)
- `loadCLIConfig()` --references--> `Config`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `loadCLIConfig()` --calls--> `Load()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `runBackupCLI()` --calls--> `OpenNoMigrate()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/store/store.go
- `runRestoreCLI()` --calls--> `RecoverIncompleteSwap()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/backup/recovery.go
- `runRestoreCLI()` --calls--> `OpenNoMigrate()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/store/store.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (133 total, 43 thin omitted)

### Community 0 - "Method"
Cohesion: 0.07
Nodes (59): candidate, expenseDetailView, fieldError, participantView, fieldErrorf(), formatValue(), lineForMethod(), orZero() (+51 more)

### Community 1 - "covUser"
Cohesion: 0.19
Nodes (19): generatedFrom(), Service, TestAddExpenseOmitsNonSplittingPayer(), TestAddExpenseRecordsTypedInputs(), TestGenerateOneCopiesWhenNoInputs(), TestGenerateOneReSplitsFromInputs(), TestMoveExpenseRejectsSettlement(), TestMoveExpenseRejectsStaleVersion() (+11 more)

### Community 2 - "net/http.ResponseWriter"
Cohesion: 0.13
Nodes (5): net/http.ResponseWriter, Server, safeNext(), Server, Server

### Community 3 - "NewManager"
Cohesion: 0.08
Nodes (41): ctxKey, net/http.Handler, CSRFFrom(), Manager, NewManager(), UserFrom(), HashPassword(), RandomToken() (+33 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.11
Nodes (33): backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults(), TestBackupDirDifferentFromAutoRestoreDirAccepted(), TestBackupDirEqualAutoRestoreDirFailsLoud(), TestBackupRetentionValidation() (+25 more)

### Community 6 - "Server"
Cohesion: 0.15
Nodes (7): displayName(), fieldErrorsFor(), formVersion(), Server, parseExpenseInput(), parseSettlementInput(), prefillFromForm()

### Community 7 - "body"
Cohesion: 0.06
Nodes (68): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestConvertExactBothAmounts(), TestRateEndpoint() (+60 more)

### Community 8 - "openTestStore"
Cohesion: 0.06
Nodes (57): Transfer, Simplify(), netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit() (+49 more)

### Community 9 - "Implementation"
Cohesion: 0.06
Nodes (30): Archive format, Context, Decided requirements, Deviations from the plan as approved, Docs, Encryption, Explicitly deferred / rejected, GoSplit backup and restore (+ surface the build version) (+22 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.13
Nodes (7): Service, Service, toISO(), todayISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "app service"
Cohesion: 0.67
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.18
Nodes (6): prefixCols(), trimSpace(), Store, scanGroup(), Group, TestCov2PureHelpers()

### Community 13 - "Format"
Cohesion: 0.18
Nodes (12): absInt64(), Server, groupSettleSuggestion(), decimals(), Format(), Parse(), pow10(), TestFormat() (+4 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "testKeyring"
Cohesion: 0.24
Nodes (20): frameAt(), seal(), TestDuplicatedFrameFails(), TestEmptyPayloadStillTerminates(), TestFingerprintDistinguishesSecrets(), TestFingerprintDoesNotRevealSealKey(), TestFingerprintIsStable(), TestFlippedFinalFlagFails() (+12 more)

### Community 16 - "nowISO"
Cohesion: 0.12
Nodes (11): time.Duration, Store, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store (+3 more)

### Community 17 - "nameCache"
Cohesion: 0.22
Nodes (9): balanceRow, currencyNet, groupRow, nameCache, settlementRow, Server, max64(), min64() (+1 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "service_test.go"
Cohesion: 0.16
Nodes (11): extractToken(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion(), TestForgotResetPassword(), TestImportFromSplitwise(), TestMagicLinkFlow(), TestRegisterAndLogin() (+3 more)

### Community 21 - "Expense"
Cohesion: 0.13
Nodes (12): FormatWithCode(), Service, BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, inPlaceholders(), scanExpense() (+4 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.11
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "newTestRunner"
Cohesion: 0.28
Nodes (20): countRows(), exec(), newTestRunner(), newTestRunnerIn(), seedEverything(), seedUploads(), wipeAll(), TestDumpIsDeterministic() (+12 more)

### Community 24 - "helpers.go"
Cohesion: 0.19
Nodes (12): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+4 more)

### Community 25 - "context.Context"
Cohesion: 0.08
Nodes (14): context.Context, Service, Service, Service, swGet(), Service, Store, Store (+6 more)

### Community 26 - "convert.js"
Cohesion: 0.36
Nodes (9): dec(), fetchRate(), fmt(), num(), recomputeTo(), setSrc(), sig6(), summarize() (+1 more)

### Community 32 - "ctxTimeout"
Cohesion: 0.09
Nodes (9): context.CancelFunc, Server, Server, applySubmittedFields(), Server, sortedNets(), Server, atoi64() (+1 more)

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
Cohesion: 0.21
Nodes (10): database/sql.NullInt64, database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullInt(), nullStr() (+2 more)

### Community 40 - "Transaction"
Cohesion: 0.12
Nodes (6): Disabled, Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.46
Nodes (6): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol()

### Community 42 - "index_test.go"
Cohesion: 0.31
Nodes (24): craftEntry, ReadHeader(), tablePartName(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive(), itoa() (+16 more)

### Community 43 - "tripHarness"
Cohesion: 0.13
Nodes (31): harness, settlementPath(), TestEditSettlementGroupMoveNeedsAck(), TestEditSettlementHidesFixedFieldPickers(), TestEditSettlementKeepsZeroDecimalCurrency(), TestEditSettlementNonMemberRendersFieldError(), TestEditSettlementPageHasNoMethodPicker(), TestEditSettlementRejectsBadAmount() (+23 more)

### Community 44 - "Store"
Cohesion: 0.22
Nodes (6): friendRow, friendSettlePrefill(), CumulatedBalance, Store, scanBalances(), Balance

### Community 45 - "testing.T"
Cohesion: 0.11
Nodes (38): testing.T, adminHarness(), harness, harness, TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted(), TestBackupDownloadRejectsHostileNames(), TestBackupDownloadServesArchive() (+30 more)

### Community 46 - "Store"
Cohesion: 0.12
Nodes (21): database/sql.DB, Config, New(), Service, TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestOpenMigratesAndOpenNoMigrateDoesNot(), TestOpenNoMigrateLeavesSchemaAlone() (+13 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.44
Nodes (9): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+1 more)

### Community 49 - "Scan"
Cohesion: 0.16
Nodes (24): countingReader, Entry, EntryKind, Index, Limits, Opener, Validated, archive/tar.Reader (+16 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.70
Nodes (4): b64ToUint8(), csrf(), enable(), test()

### Community 53 - "newTestService"
Cohesion: 0.13
Nodes (21): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestCreateConversionExact(), TestAddFriendToGroup(), Service, newTestService(), TestAddExpenseDirectCreatesFriendship(), TestAddExpenseNonMemberRejected() (+13 more)

### Community 54 - "service/zz_coverage2_test.go"
Cohesion: 0.18
Nodes (9): covMultipartReq(), TestCovBankConnectedFlow(), TestCovBankProviderErrors(), TestCovConversionErrors(), TestCovEmailParticipants(), TestCovInviteSendsPendingInvites(), TestCovPushGating(), TestCovUpdateAvatar() (+1 more)

### Community 55 - "harness"
Cohesion: 0.18
Nodes (11): net/http/httptest.Server, net/http.Response, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect() (+3 more)

### Community 57 - "PlaidProvider"
Cohesion: 0.16
Nodes (14): PlaidProvider, roundTripFunc, Provider, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken() (+6 more)

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - ".handleGroupDetail"
Cohesion: 0.23
Nodes (13): expenseRow, monthGroup, TestFlashCookieRoundTrip(), TestTakeFlashClearsCookie(), applyFeedLimit(), dateOnly(), groupByMonth(), monthLabel() (+5 more)

### Community 62 - "Renderer"
Cohesion: 0.26
Nodes (3): html/template.Template, Renderer, ViewData

### Community 63 - "Bundle"
Cohesion: 0.25
Nodes (5): Bundle, Load(), parseAcceptLanguage(), parseQ(), TestTemplateKeysResolve()

### Community 64 - "seedDirectExpense"
Cohesion: 0.15
Nodes (24): harness, seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord() (+16 more)

### Community 66 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.08
Nodes (103): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+95 more)

### Community 82 - "JobTracker"
Cohesion: 0.07
Nodes (41): JobState, pending, sync/atomic.Bool, sync.Mutex, time.Time, JobStatus, JobTracker, NewJobTracker() (+33 more)

### Community 83 - "Keyring"
Cohesion: 0.13
Nodes (19): Keyring, openReader, sealWriter, crypto/cipher.AEAD, io.WriteCloser, io.Writer, expand(), newAEAD() (+11 more)

### Community 84 - "GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural)"
Cohesion: 0.18
Nodes (10): Context, Deviations from the approved plan, Docs (same change), Explicitly deferred / rejected, GoSplit responsiveness + UX plan (Tier 1 quick wins + Tier 2 structural), Ledger, Tests (same change as the code they cover), Tier 1 -- quick wins (independent, ship in any order) (+2 more)

### Community 85 - "RecoverIncompleteSwap"
Cohesion: 0.19
Nodes (16): ApplyOptions, dirExists(), RecoverIncompleteSwap(), marker(), swapFixture(), TestRecoverCleanupOnlyCrash(), TestRecoverIsIdempotent(), TestRecoverMidSwapCrash() (+8 more)

### Community 87 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 88 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 89 - "InsertOrder"
Cohesion: 0.11
Nodes (27): batcher, Column, ColumnKind, Table, database/sql.Tx, ScanDest(), snapshotTables(), DeleteOrder() (+19 more)

### Community 90 - "run"
Cohesion: 0.31
Nodes (7): main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion(), wantsVersion(), log/slog.Level

### Community 91 - "net/http.Request"
Cohesion: 0.16
Nodes (6): net/http.Request, archiveInfo, Server, writeJobJSON(), Server, navSlug()

### Community 92 - "Header"
Cohesion: 0.42
Nodes (4): Header, TableStat, decodeB64(), WriteHeader()

### Community 93 - ".Dump"
Cohesion: 0.20
Nodes (10): queryer, archive/tar.Writer, Manifest, b64(), Runner, readMigrationVersions(), SameMigrationSet(), writeTarFile() (+2 more)

### Community 94 - "New"
Cohesion: 0.16
Nodes (22): cliContext(), humanBytes(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI() (+14 more)

### Community 95 - "codec_test.go"
Cohesion: 0.17
Nodes (21): database/sql.NullBool, encoding/json.RawMessage, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), nullBool(), nullString() (+13 more)

### Community 96 - "CheckAutoRestore"
Cohesion: 0.43
Nodes (7): Report, acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), orDashStr(), probeWritable(), writeMarker()

### Community 97 - "New"
Cohesion: 0.18
Nodes (9): New(), TestCovAuthErrorBranches(), TestCovDispatchDefaults(), TestCovEmailParticipantsSendError(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovPushEnabledSendError(), TestCovRecurrenceErrorBranches() (+1 more)

### Community 98 - "load"
Cohesion: 0.52
Nodes (6): firstOther(), load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate()

### Community 99 - "nullInt"
Cohesion: 0.24
Nodes (8): nullInt(), TestDeleteExpenseAuthorization(), TestDeleteExpenseRejectsStaleVersion(), TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted(), TestMoveExpenseUnauthorized()

### Community 127 - "NewRenderer"
Cohesion: 0.62
Nodes (6): feedViewData(), TestFragmentMatchesRegionInFullPage(), TestRenderFragmentOmitsLayout(), TestRenderFragmentSetsNoStoreHeaders(), TestRenderFragmentUnknownFailsLoudly(), NewRenderer()

### Community 129 - ".serveAsset"
Cohesion: 0.50
Nodes (3): net/http.HandlerFunc, Asset(), TestCovAsset()

### Community 130 - "TestActivityFeedShowAll"
Cohesion: 0.40
Nodes (4): TestActivityFeedShowAll(), TestFeedDateParts(), TestGroupByMonth(), feedDateParts()

### Community 131 - "TestCov3BankConvertRedirect"
Cohesion: 0.60
Nodes (4): covSeedBankTx(), harness, TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

## Knowledge Gaps
- **132 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+127 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **43 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Open()` connect `Store` to `InsertOrder`, `NewManager`, `body`, `openTestStore`, `JobTracker`, `newTestService`, `newTestRunner`, `context.Context`, `run`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `net/http.Request`, `testing.T`?**
  _High betweenness centrality (0.031) - this node is a cross-community bridge._
- **Why does `newHarness()` connect `body` to `seedDirectExpense`, `New`, `TestActivityFeedShowAll`, `NewManager`, `TestCov3BankConvertRedirect`, `tripHarness`, `testing.T`, `Store`, `TestThemeColorUpdate`, `harness`, `NewRenderer`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Are the 113 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 113 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _132 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Method` be split into smaller, more focused modules?**
  _Cohesion score 0.0673903211216644 - nodes in this community are weakly interconnected._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.12666666666666668 - nodes in this community are weakly interconnected._