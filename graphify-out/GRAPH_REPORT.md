# Graph Report - gosplit  (2026-09-07)

## Corpus Check
- 196 files · ~182,905 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2080 nodes · 5757 edges · 147 communities (87 shown, 45 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 1046 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2b67d694`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.ResponseWriter
- newTestService
- ctxTimeout
- NewManager
- Base Layout
- Config
- net/http.Request
- body
- openTestStore
- provider_test.go
- ExpenseRecurrence
- configuration.md
- Group
- Format
- Group Detail Page
- testKeyring
- nowISO
- codec_test.go
- database/sql query layer (as-built)
- dev.sh
- service_test.go
- Expense
- currency/zz_coverage_test.go
- newTestRunner
- Bundle
- context.Context
- convert.js
- collapseView
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- Method
- Auth (magic-link + password)
- SQLite default (no DB container)
- .ImportFromSplitwise
- Currency conversion (pluggable providers)
- dev.ps1
- Features
- Service
- Transaction
- expense_form.js
- index_test.go
- tripHarness
- Store
- testing.T
- RandomToken
- Convert
- mkExp
- Scan
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- paths_cli_test.go
- service/zz_coverage2_test.go
- harness
- handlers_recurring.go
- New
- bank.js
- GoSplit App Icon
- bankTxRow
- .handleGroupDetail
- Development
- service/move_test.go
- seedDirectExpense
- sw.js
- Architecture
- htmx.min.js
- github.com/hafio/gosplit
- JobTracker
- crypto.go
- Configuration
- Deployment
- TestThemeColorUpdate
- Backup and restore
- nameCache
- Store
- Server
- Manifest
- .Dump
- PlaidProvider
- .buildBatches
- helpers.go
- service/zz_coverage3_test.go
- CheckAutoRestore
- New
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
- Client behaviour
- Validated
- Restoring
- RecoverIncompleteSwap
- TestCov3BankConvertRedirect
- Renderer
- GoSplit documentation
- run
- Server
- web/zz_coverage_test.go
- newHarness
- view.go
- NewRenderer
- ValidTheme
- Store
- ownerOf
- TestCov2PureHelpers
- TestAdminCreateUser
- TestDeleteExpenseAuthorization
- TestAddFriendToGroup

## God Nodes (most connected - your core abstractions)
1. `body()` - 130 edges
2. `newHarness()` - 106 edges
3. `newTestService()` - 73 edges
4. `ctxTimeout()` - 64 edges
5. `Expense` - 47 edges
6. `covUser()` - 45 edges
7. `User` - 43 edges
8. `openTestStore()` - 41 edges
9. `testKeyring()` - 35 edges
10. `Config` - 35 edges

## Surprising Connections (you probably didn't know these)
- `app service` --conceptually_related_to--> `SplitPro (Go rebuild)`  [INFERRED]
  docker-compose.yml → README.md
- `loadCLIConfig()` --references--> `Config`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `loadCLIConfig()` --calls--> `Load()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go
- `runBackupCLI()` --calls--> `OpenNoMigrate()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/store/store.go
- `runRestoreCLI()` --calls--> `RecoverIncompleteSwap()`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/backup/recovery.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (147 total, 45 thin omitted)

### Community 0 - "net/http.ResponseWriter"
Cohesion: 0.11
Nodes (6): net/http.ResponseWriter, Server, safeNext(), Server, Server, Server

### Community 1 - "newTestService"
Cohesion: 0.13
Nodes (37): TestCreateConversionExact(), Service, newTestService(), generatedFrom(), Service, TestAddExpenseOmitsNonSplittingPayer(), TestAddExpenseRecordsTypedInputs(), TestGenerateOneCopiesWhenNoInputs() (+29 more)

### Community 2 - "ctxTimeout"
Cohesion: 0.15
Nodes (4): context.CancelFunc, Server, sortedNets(), ctxTimeout()

### Community 3 - "NewManager"
Cohesion: 0.11
Nodes (34): ctxKey, net/http.Handler, archiveInfo, CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser() (+26 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Config"
Cohesion: 0.10
Nodes (35): backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults(), TestBackupDirDifferentFromAutoRestoreDirAccepted(), TestBackupDirEqualAutoRestoreDirFailsLoud(), TestBackupRetentionValidation() (+27 more)

### Community 6 - "net/http.Request"
Cohesion: 0.17
Nodes (11): net/http.Request, Server, applySubmittedFields(), displayName(), fieldErrorsFor(), formVersion(), Server, parseExpenseInput() (+3 more)

### Community 7 - "body"
Cohesion: 0.10
Nodes (41): body(), covPostJSON(), harness, TestCov2BankDisabledErrors(), TestCov2ConvertPagePreselectsBalance(), TestCov2ConvertSuccess(), TestCov2FriendCollapse(), TestCov2GroupCollapseError() (+33 more)

### Community 8 - "openTestStore"
Cohesion: 0.07
Nodes (54): BuildLikePattern(), TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit(), TestGroupMemberCountsAndNets(), TestListFriendExpensesFilters(), TestAdminUpdateUser(), TestListFriendExpensesIncludesGroups() (+46 more)

### Community 9 - "provider_test.go"
Cohesion: 0.29
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.18
Nodes (5): Service, toISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "configuration.md"
Cohesion: 0.60
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.24
Nodes (3): Store, scanGroup(), Group

### Community 13 - "Format"
Cohesion: 0.13
Nodes (12): Server, absInt64(), Server, decimals(), Format(), Parse(), pow10(), TestFormat() (+4 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "testKeyring"
Cohesion: 0.16
Nodes (27): Keyring, io.WriteCloser, newAEAD(), NewOpenReader(), NewSealWriter(), frameAt(), open(), seal() (+19 more)

### Community 16 - "nowISO"
Cohesion: 0.14
Nodes (10): time.Duration, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store, Store (+2 more)

### Community 17 - "codec_test.go"
Cohesion: 0.17
Nodes (22): database/sql.NullBool, encoding/json.RawMessage, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), nullBool(), nullInt() (+14 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "service_test.go"
Cohesion: 0.16
Nodes (11): extractToken(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion(), TestForgotResetPassword(), TestImportFromSplitwise(), TestMagicLinkFlow(), TestRegisterAndLogin() (+3 more)

### Community 21 - "Expense"
Cohesion: 0.15
Nodes (12): database/sql.Tx, FormatWithCode(), Service, ExpenseFilter, HistoricalBatch, Store, inPlaceholders(), scanExpense() (+4 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.11
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "newTestRunner"
Cohesion: 0.08
Nodes (55): batcher, Column, ColumnKind, Table, countRows(), exec(), newTestRunner(), newTestRunnerIn() (+47 more)

### Community 24 - "Bundle"
Cohesion: 0.18
Nodes (11): Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load(), TestDetect(), TestLanguages() (+3 more)

### Community 25 - "context.Context"
Cohesion: 0.08
Nodes (12): context.Context, Service, Service, Service, Store, Store, Store, User (+4 more)

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 32 - "Method"
Cohesion: 0.07
Nodes (59): candidate, expenseDetailView, fieldError, participantView, fieldErrorf(), formatValue(), lineForMethod(), orZero() (+51 more)

### Community 34 - "SQLite default (no DB container)"
Cohesion: 0.67
Nodes (3): Postgres db service (optional, commented), SQLite default (no DB container), SQLite (pure-Go modernc.org/sqlite)

### Community 35 - ".ImportFromSplitwise"
Cohesion: 0.47
Nodes (3): Service, swGet(), ImportResult

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.11
Nodes (19): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+11 more)

### Community 39 - "Service"
Cohesion: 0.22
Nodes (10): database/sql.NullInt64, database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullInt(), nullStr() (+2 more)

### Community 40 - "Transaction"
Cohesion: 0.18
Nodes (5): Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

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
Cohesion: 0.14
Nodes (27): testing.T, TestActivityFeedShowAll(), TestFeedDateParts(), TestGroupByMonth(), adminHarness(), harness, harness, TestAdminPageLinksToBackup() (+19 more)

### Community 46 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 47 - "Convert"
Cohesion: 0.36
Nodes (7): math/big.Rat, Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "mkExp"
Cohesion: 0.44
Nodes (9): countRows(), Service, mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+1 more)

### Community 49 - "Scan"
Cohesion: 0.17
Nodes (23): countingReader, Entry, EntryKind, Index, Limits, Opener, archive/tar.Reader, io.Reader (+15 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.57
Nodes (6): b64ToUint8(), csrf(), enable(), test(), wire(), wireOnce()

### Community 53 - "paths_cli_test.go"
Cohesion: 0.15
Nodes (26): prepareDataDirs(), collectDirStatuses(), inspectDir(), printDirContents(), printDirStatus(), probeDir(), runPathsCLI(), captureStdout() (+18 more)

### Community 54 - "service/zz_coverage2_test.go"
Cohesion: 0.18
Nodes (9): covMultipartReq(), TestCovBankConnectedFlow(), TestCovBankProviderErrors(), TestCovConversionErrors(), TestCovEmailParticipants(), TestCovInviteSendsPendingInvites(), TestCovPushGating(), TestCovUpdateAvatar() (+1 more)

### Community 55 - "harness"
Cohesion: 0.18
Nodes (11): net/http/httptest.Server, net/http.Response, net/url.Values, harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect() (+3 more)

### Community 57 - "New"
Cohesion: 0.21
Nodes (18): cliContext(), humanBytes(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI() (+10 more)

### Community 58 - "bank.js"
Cohesion: 0.83
Nodes (3): connect(), csrf(), wire()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 61 - ".handleGroupDetail"
Cohesion: 0.30
Nodes (8): expenseRow, monthGroup, applyFeedLimit(), dateOnly(), groupByMonth(), monthLabel(), parseFilter(), showAllHref()

### Community 62 - "Development"
Cohesion: 0.11
Nodes (18): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Cutting a release, Dev scripts (+10 more)

### Community 63 - "service/move_test.go"
Cohesion: 0.33
Nodes (5): TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted(), TestMoveExpenseUnauthorized()

### Community 64 - "seedDirectExpense"
Cohesion: 0.15
Nodes (24): harness, seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord() (+16 more)

### Community 66 - "Architecture"
Cohesion: 0.12
Nodes (16): Architecture, Background jobs, Backup archives, Balances are derived, never stored, Collapsing history, Data model, Decisions worth keeping, Internationalisation (+8 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.06
Nodes (118): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+110 more)

### Community 82 - "JobTracker"
Cohesion: 0.09
Nodes (33): JobState, pending, sync.Mutex, time.Time, JobStatus, JobTracker, NewJobTracker(), newToken() (+25 more)

### Community 83 - "crypto.go"
Cohesion: 0.18
Nodes (11): openReader, sealWriter, crypto/cipher.AEAD, expand(), NewKeyring(), NewNoncePrefix(), NewSalt(), nonceFor() (+3 more)

### Community 84 - "Configuration"
Cohesion: 0.13
Nodes (15): Accounts and features, Backup and restore, Bank sync (Plaid, optional), Configuration, Core, Currency rates, Database, Email (SMTP) (+7 more)

### Community 85 - "Deployment"
Cohesion: 0.13
Nodes (15): Bind mounts must be owned by 65532, Choosing an engine, Deployment, Docker Compose, First admin, Health check, Mail, Monitoring and logs (+7 more)

### Community 87 - "Backup and restore"
Cohesion: 0.14
Nodes (14): Archives are secrets, Backup and restore, Four ways to make a backup, From the admin page, From the command line, Housekeeping, Inspecting an archive, Not supported, by decision (+6 more)

### Community 88 - "nameCache"
Cohesion: 0.14
Nodes (15): Transfer, balanceRow, currencyNet, groupRow, nameCache, settlementRow, Simplify(), netAfter() (+7 more)

### Community 89 - "Store"
Cohesion: 0.14
Nodes (18): database/sql.DB, TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestOpenMigratesAndOpenNoMigrateDoesNot(), TestOpenNoMigrateLeavesSchemaAlone(), TestPendingMigrationsOnEmptyDatabase(), TestRebindMatchesUnexported(), TestRebindRewritesForPostgresOnly() (+10 more)

### Community 92 - "Manifest"
Cohesion: 0.26
Nodes (6): Header, TableStat, io.Writer, decodeB64(), Manifest, WriteHeader()

### Community 93 - ".Dump"
Cohesion: 0.29
Nodes (8): queryer, archive/tar.Writer, ScanDest(), b64(), Runner, readMigrationVersions(), writeTarFile(), TestHeaderRejectsAbsurdKDFParameters()

### Community 94 - "PlaidProvider"
Cohesion: 0.17
Nodes (3): Disabled, PlaidProvider, Provider

### Community 95 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 96 - "helpers.go"
Cohesion: 0.16
Nodes (14): avatarColor(), categories(), categoryEmoji(), currencyCodes(), firstAlnum(), initials(), methodGlyph(), TestAvatarColorStableAndInRange() (+6 more)

### Community 97 - "service/zz_coverage3_test.go"
Cohesion: 0.22
Nodes (7): TestCovAuthErrorBranches(), TestCovEmailParticipantsSendError(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovPushEnabledSendError(), TestCovRecurrenceErrorBranches(), covFailMailer

### Community 98 - "CheckAutoRestore"
Cohesion: 0.60
Nodes (5): acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), orDashStr(), writeMarker()

### Community 99 - "New"
Cohesion: 0.05
Nodes (51): crypto/tls.Config, net.Listener, net/smtp.Client, sync/atomic.Bool, buildMessage(), Mailer, New(), serveFakeSMTP() (+43 more)

### Community 127 - "Client behaviour"
Cohesion: 0.29
Nodes (7): Client behaviour, Fragments, Freshness, Measurement, Navigation, Progressive web app, Safeguards

### Community 128 - "Validated"
Cohesion: 0.21
Nodes (7): ApplyOptions, Report, Validated, SameMigrationSet(), Runner, InspectFile(), TestSameMigrationSet()

### Community 129 - "Restoring"
Cohesion: 0.33
Nodes (6): Check first, Migration sets must match exactly, Replace everything, Restoring, Restoring from the admin page, While a restore runs

### Community 130 - "RecoverIncompleteSwap"
Cohesion: 0.34
Nodes (14): dirExists(), RecoverIncompleteSwap(), marker(), swapFixture(), TestRecoverCleanupOnlyCrash(), TestRecoverIsIdempotent(), TestRecoverMidSwapCrash(), TestRecoverMidSwapCrashWithoutStaging() (+6 more)

### Community 131 - "TestCov3BankConvertRedirect"
Cohesion: 0.60
Nodes (4): covSeedBankTx(), harness, TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 132 - "Renderer"
Cohesion: 0.26
Nodes (5): html/template.Template, Renderer, ViewData, htmlHeaders(), setServerTiming()

### Community 133 - "GoSplit documentation"
Cohesion: 0.67
Nodes (3): All pages, GoSplit documentation, Start here

### Community 134 - "run"
Cohesion: 0.23
Nodes (10): TestWantsBackup(), TestWantsBackupAndVersionAreDisjoint(), wantsBackup(), main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion() (+2 more)

### Community 135 - "Server"
Cohesion: 0.21
Nodes (5): fragmentTarget(), Server, isBackgroundRequest(), navSlug(), New()

### Community 136 - "web/zz_coverage_test.go"
Cohesion: 0.21
Nodes (12): net/http.HandlerFunc, Asset(), covRenderer(), TestCovAsset(), TestCovAssetsHandlerCacheControl(), TestCovFuncsAbs64(), TestCovFuncsDict(), TestCovRendererLanguages() (+4 more)

### Community 137 - "newHarness"
Cohesion: 0.06
Nodes (40): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestConvertExactBothAmounts(), TestRateEndpoint() (+32 more)

### Community 138 - "view.go"
Cohesion: 0.16
Nodes (12): TestAssetURLFingerprinted(), TestQueryWithout(), TestFragmentForLookup(), TestPollableCoversListPagesOnly(), assetURL(), csvRows(), FragmentFor(), pollable() (+4 more)

### Community 139 - "NewRenderer"
Cohesion: 0.42
Nodes (10): feedViewData(), TestFragmentMatchesRegionInFullPage(), TestFragmentRegistryIsValid(), TestFragmentVersionTracksContent(), TestRenderFragmentOmitsLayout(), TestRenderFragmentReportsTemplateFailure(), TestRenderFragmentSetsNoStoreHeaders(), TestRenderFragmentUnknownFailsLoudly() (+2 more)

### Community 140 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 142 - "ownerOf"
Cohesion: 0.40
Nodes (3): ownerOf(), ownerOf(), io/fs.FileInfo

### Community 143 - "TestCov2PureHelpers"
Cohesion: 0.83
Nodes (3): prefixCols(), trimSpace(), TestCov2PureHelpers()

## Knowledge Gaps
- **190 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+185 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 295 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **45 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `testing.T`, `net/http.Request`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **Why does `Open()` connect `Store` to `newTestService`, `New`, `NewManager`, `Config`, `run`, `openTestStore`, `newHarness`, `newTestRunner`, `context.Context`?**
  _High betweenness centrality (0.034) - this node is a cross-community bridge._
- **Why does `User` connect `context.Context` to `newTestService`, `NewManager`, `Renderer`, `.ImportFromSplitwise`, `net/http.Request`, `Server`, `Service`, `openTestStore`, `Transaction`, `Store`, `RandomToken`, `Expense`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._
- **Are the 120 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 120 INFERRED edges - model-reasoned connections that need verification._
- **Are the 93 inferred relationships involving `newHarness()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`newHarness()` has 93 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _190 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.10837438423645321 - nodes in this community are weakly interconnected._