# Graph Report - goSplit  (2026-10-20)

## Corpus Check
- 222 files · ~221,016 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .example 1, .css 1)

## Summary
- 2427 nodes · 10183 edges · 132 communities (90 shown, 42 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1235 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `aaf6ac9b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.ResponseWriter
- testing.T
- idiomorph-ext.min.js
- NewManager
- Base Layout
- Load
- atoi64
- .Dump
- openTestStore
- provider_test.go
- ExpenseRecurrence
- configuration.md
- Group
- Format
- Group Detail Page
- crypto_test.go
- context.Context
- EncodeColumn
- database/sql query layer (as-built)
- dev.sh
- render_fragment_test.go
- Store
- currency/zz_coverage_test.go
- roundtrip_test.go
- index.go
- User
- convert.js
- .handleAdminActivity
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- split_test.go
- Auth (magic-link + password)
- SQLite default (no DB container)
- SMTPMailer
- Currency conversion (pluggable providers)
- dev.ps1
- Features
- Expense
- Transaction
- expense_form.js
- index_test.go
- tripHarness
- Store
- handlers_backup_test.go
- RandomToken
- convert.go
- covfilter/main.go
- Header
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- inspectDir
- Renderer
- harness
- gosplit/main.go
- backup_cli_test.go
- bank.js
- GoSplit App Icon
- Service
- .netGroup
- Development
- NewJobTracker
- go_pkg_net_url
- sw.js
- Architecture
- .buildNotificationRows
- web/theme_test.go
- newChain
- BackupSchedule
- Client behaviour
- ExpenseParticipant
- group_access_test.go
- userByEmail
- net/http.Handler
- NewUUID
- runRestoreCLI
- TestCov2PureHelpers
- main
- htmx.min.js
- github.com/hafio/gosplit
- Validated
- crypto.go
- Configuration
- Deployment
- printDirContents
- Backup and restore
- rewire_test.go
- serveFakeSMTP
- fakeRates
- New
- .CreateConversionExact
- SweepStalePartials
- PlaidProvider
- looksLikePostgresKeywordDSN
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
- Split engine (internal/split)
- Splitwise import
- Web Push notifications (VAPID)
- prune_test.go
- Store
- EnsureWritableDir
- web/zz_coverage_test.go
- Config
- net/http.Request
- body
- push_test.go
- ownerOf

## God Nodes (most connected - your core abstractions)
1. `body()` - 167 edges
2. `newHarness()` - 123 edges
3. `newTestService()` - 89 edges
4. `ctxTimeout()` - 71 edges
5. `Expense` - 62 edges
6. `User` - 59 edges
7. `covUser()` - 53 edges
8. `openTestStore()` - 51 edges
9. `testKeyring()` - 35 edges
10. `Config` - 35 edges

## Surprising Connections (you probably didn't know these)
- `How settings are read` --references--> `F()`  [INFERRED]
  docs/configuration.md → internal/web/assets/htmx.min.js
- `How settings are read` --references--> `f()`  [INFERRED]
  docs/configuration.md → internal/web/assets/idiomorph-ext.min.js
- `GitHub Actions` --references--> `X()`  [INFERRED]
  docs/development.md → internal/web/assets/htmx.min.js
- `Versioning and release` --references--> `v()`  [INFERRED]
  docs/development.md → internal/web/assets/htmx.min.js
- `Scan()` --calls--> `opener`  [INFERRED]
  internal/backup/index.go → scripts/covfilter/main.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (132 total, 42 thin omitted)

### Community 0 - "net/http.ResponseWriter"
Cohesion: 0.11
Nodes (8): Server, Server, Server, sortedNets(), Server, ctxTimeout(), ValidRole(), TestCovModelFlags()

### Community 1 - "testing.T"
Cohesion: 0.04
Nodes (120): nullInt(), TestIsAdminEmail(), TestLogMailerSend(), TestSMTPMailerDialError(), newOverrideFixture(), TestAdminEditsAndDeletesAnyTransaction(), TestAdminGetsNoGroupLevelPowers(), TestAdminMoveSkipsOnlyTheActorCheck() (+112 more)

### Community 2 - "idiomorph-ext.min.js"
Cohesion: 0.28
Nodes (15): a(), c(), d(), e(), f(), h(), i(), l() (+7 more)

### Community 3 - "NewManager"
Cohesion: 0.18
Nodes (24): CSRFFrom(), NewManager(), UserFrom(), covNewUser(), covOKHandler(), covOpenStore(), TestCovAuthenticate_ExistingCSRFReused(), TestCovAuthenticate_InactiveUserNotLoaded() (+16 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.10
Nodes (25): backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults(), TestBackupDirDifferentFromAutoRestoreDirAccepted(), TestBackupDirEqualAutoRestoreDirFailsLoud(), TestBackupRetentionValidation() (+17 more)

### Community 6 - "atoi64"
Cohesion: 0.10
Nodes (19): candidate, fieldError, applySubmittedFields(), displayName(), fieldErrorf(), fieldErrorsFor(), formVersion(), Server (+11 more)

### Community 7 - ".Dump"
Cohesion: 0.25
Nodes (6): queryer, b64(), Runner, readMigrationVersions(), writeTarFile(), TestHeaderRejectsAbsurdKDFParameters()

### Community 8 - "openTestStore"
Cohesion: 0.06
Nodes (55): amountPtr(), expenseNames(), nullGroup(), TestListAllActivity(), TestListAllGroupsAndCounts(), TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestUserNetByExpense() (+47 more)

### Community 9 - "provider_test.go"
Cohesion: 0.29
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.20
Nodes (6): Service, toISO(), todayISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "configuration.md"
Cohesion: 0.41
Nodes (6): app service, splitpro-data volume, All pages, GoSplit documentation, Start here, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.15
Nodes (9): currencyNet, groupListRow, groupRow, hasGroup(), Service, Store, scanGroup(), Group (+1 more)

### Community 13 - "Format"
Cohesion: 0.16
Nodes (12): absInt64(), Server, friendSettlePrefill(), groupSettleSuggestion(), decimals(), Format(), Parse(), pow10() (+4 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "crypto_test.go"
Cohesion: 0.16
Nodes (26): Keyring, newAEAD(), NewOpenReader(), NewSealWriter(), frameAt(), open(), seal(), TestDuplicatedFrameFails() (+18 more)

### Community 16 - "context.Context"
Cohesion: 0.05
Nodes (21): NoneProvider, Server, Store, Store, Store, FromNow(), nowISO(), Store (+13 more)

### Community 17 - "EncodeColumn"
Cohesion: 0.38
Nodes (6): EncodeColumn(), nullBool(), nullString(), TestDecodeColumnRoundTrip(), TestEncodeColumn(), TestEncodeColumnRejectsUnexpectedNull()

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "render_fragment_test.go"
Cohesion: 0.09
Nodes (29): TestNotificationKindsHaveLocaleEntries(), Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load(), TestDetect() (+21 more)

### Community 21 - "Store"
Cohesion: 0.18
Nodes (6): BuildLikePattern(), ExpenseFilter, Store, scanExpense(), TestBuildLikePattern(), GroupScope

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.16
Nodes (23): covRoundTripFunc, defaultClient(), Provider, NewProvider(), covClient(), covJSONResp(), TestFrankfurterRateMalformedJSON(), TestFrankfurterRateMissingRate() (+15 more)

### Community 23 - "roundtrip_test.go"
Cohesion: 0.06
Nodes (72): batcher, Column, ColumnKind, Table, DecodeColumn(), isJSONNull(), isJSONSpace(), ScanDest() (+64 more)

### Community 24 - "index.go"
Cohesion: 0.14
Nodes (20): countingReader, Entry, EntryKind, Index, Limits, Opener, decodeRows(), drainCapped() (+12 more)

### Community 25 - "User"
Cohesion: 0.11
Nodes (6): Service, Service, User, Store, scanUser(), ImportResult

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 27 - ".handleAdminActivity"
Cohesion: 0.16
Nodes (11): expenseRow, monthGroup, TestGroupByMonth(), Server, applyFeedLimit(), dateOnly(), groupByMonth(), monthLabel() (+3 more)

### Community 32 - "split_test.go"
Cohesion: 0.08
Nodes (42): formatValue(), lineForMethod(), orZero(), prefillStored(), rebalanceToTarget(), sumTarget(), valueForMethod(), TestPrefillStoredKeepsCompleteSet() (+34 more)

### Community 35 - "SMTPMailer"
Cohesion: 0.33
Nodes (3): buildMessage(), TestBuildMessage(), SMTPMailer

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.10
Nodes (20): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+12 more)

### Community 39 - "Expense"
Cohesion: 0.16
Nodes (14): expenseDetailView, participantView, ExpenseInput, Service, SettlementInput, Immutable(), logAdminOverride(), Movable() (+6 more)

### Community 40 - "Transaction"
Cohesion: 0.09
Nodes (7): Disabled, Provider, Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

### Community 42 - "index_test.go"
Cohesion: 0.31
Nodes (24): craftEntry, ReadHeader(), tablePartName(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive(), itoa() (+16 more)

### Community 43 - "tripHarness"
Cohesion: 0.10
Nodes (38): expenseByName(), TestAdminActivityFilters(), TestAdminActivityListsEveryoneNeutrally(), TestAdminActivityShowAllKeepsFilters(), TestAdminEditsDirectExpenseUntouched(), TestBuildRowsViewerZeroIsNeutral(), TestDeleteStoreFailureIsReported(), TestEditFormKeepsArchivedCurrentGroup() (+30 more)

### Community 44 - "Store"
Cohesion: 0.18
Nodes (7): friendRow, CumulatedBalance, Store, scanBalances(), Store, Balance, PlannedExpense

### Community 45 - "handlers_backup_test.go"
Cohesion: 0.12
Nodes (21): adminHarness(), harness, TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted(), TestBackupDownloadRejectsHostileNames(), TestBackupDownloadServesArchive(), TestBackupGenerateAndDownloadRoundTrip(), TestBackupJobStatusUnknownToken() (+13 more)

### Community 46 - "RandomToken"
Cohesion: 0.15
Nodes (9): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+1 more)

### Community 47 - "convert.go"
Cohesion: 0.31
Nodes (6): Convert(), decimals(), ratPow10(), roundRat(), TestConvert(), TestConvertErrors()

### Community 48 - "covfilter/main.go"
Cohesion: 0.12
Nodes (26): block, marker, pos, span, unit, applyMarkers(), directive(), main() (+18 more)

### Community 49 - "Header"
Cohesion: 0.52
Nodes (3): Header, decodeB64(), WriteHeader()

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.57
Nodes (6): b64ToUint8(), csrf(), enable(), test(), wire(), wireOnce()

### Community 53 - "inspectDir"
Cohesion: 0.25
Nodes (8): inspectDir(), probeDir(), TestInspectDirEmptyPath(), TestInspectDirMissing(), TestInspectDirNotADirectory(), TestInspectDirUnwritable(), TestInspectDirWritable(), TestProbeDirLeavesNothing()

### Community 54 - "Renderer"
Cohesion: 0.26
Nodes (4): Renderer, ViewData, htmlHeaders(), setServerTiming()

### Community 55 - "harness"
Cohesion: 0.16
Nodes (7): harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders(), TestStaticCacheHeaders()

### Community 56 - "gosplit/main.go"
Cohesion: 0.17
Nodes (15): ctxKey, balanceRow, bankTxRow, collapseView, filterOpt, recurrenceRow, templateOption, TestAssetURLFingerprinted() (+7 more)

### Community 57 - "backup_cli_test.go"
Cohesion: 0.25
Nodes (10): humanBytes(), orDash(), printManifest(), runInspectCLI(), TestHumanBytes(), TestInspectRejectsNonArchive(), TestOrDash(), TestRunInspectCLIReportsMissingFile() (+2 more)

### Community 58 - "bank.js"
Cohesion: 0.83
Nodes (3): connect(), csrf(), wire()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 60 - "Service"
Cohesion: 0.18
Nodes (9): parseLogLevel(), run(), New(), Mailer, New(), TestNewLogMailerWhenSMTPUnset(), TestNewSMTPMailerWhenHostSet(), Service (+1 more)

### Community 61 - ".netGroup"
Cohesion: 0.24
Nodes (4): nullStr(), Service, Service, nullableString()

### Community 62 - "Development"
Cohesion: 0.13
Nodes (15): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Dev scripts, Development (+7 more)

### Community 63 - "NewJobTracker"
Cohesion: 0.28
Nodes (9): NewJobTracker(), TestDiscardPendingReleases(), TestJobTrackerLifecycle(), TestJobTrackerRecordsFailure(), TestJobTrackerUnknownToken(), TestPruneDropsExpiredJobsAndPending(), TestPruneKeepsRunningJobs(), TestTakePendingIsSingleUse() (+1 more)

### Community 64 - "go_pkg_net_url"
Cohesion: 0.13
Nodes (24): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+16 more)

### Community 66 - "Architecture"
Cohesion: 0.11
Nodes (19): Architecture, Background jobs, Backup archives, Balances are derived, never stored, Collapsing history, Data model, Decisions worth keeping, Internationalisation (+11 more)

### Community 67 - ".buildNotificationRows"
Cohesion: 0.21
Nodes (6): notificationRow, entityHref(), Server, TestNotificationEntityHrefFallback(), FormatWithCode(), TestFormatWithCode()

### Community 68 - "web/theme_test.go"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 69 - "newChain"
Cohesion: 0.27
Nodes (16): assertNettingZeroSum(), assertSimplified(), nettingRows(), newChain(), notifCountFor(), pairsOf(), TestConcurrentNettingConverges(), TestDeleteGroupSettlementReNets() (+8 more)

### Community 70 - "BackupSchedule"
Cohesion: 0.33
Nodes (4): TestBackupScheduleParserMatchesRecurrences(), TestSameDir(), BackupSchedule(), sameDir()

### Community 71 - "Client behaviour"
Cohesion: 0.29
Nodes (7): Client behaviour, Fragments, Freshness, Measurement, Navigation, Progressive web app, Safeguards

### Community 72 - "ExpenseParticipant"
Cohesion: 0.35
Nodes (5): Service, notificationsFor(), TestNotificationsForSkipsActorAndEmpty(), ExpenseParticipant, Notification

### Community 73 - "group_access_test.go"
Cohesion: 0.27
Nodes (9): countOf(), groupFlags(), outsiderHarness(), TestGroupActionsRejectOutsider(), TestGroupMembersHiddenFromOutsider(), nettingBetween(), settledChainHarness(), TestNettingEntryRefusesChanges() (+1 more)

### Community 74 - "userByEmail"
Cohesion: 0.21
Nodes (15): limitedHarness(), TestAdminConsoleLimitedRole(), TestLimitedCannotCreateGroup(), TestLimitedPagesHideGroupCreationAndImport(), reread(), seedNotification(), TestBellBadgeAppearsOnEveryPage(), TestNotificationBadgeAndMenuFragments() (+7 more)

### Community 75 - "net/http.Handler"
Cohesion: 0.17
Nodes (8): Manager, maxUploadBytes(), privateResponses(), Server, isRestorePoll(), isSafeMethod(), cacheControl(), requestLogger()

### Community 76 - "NewUUID"
Cohesion: 0.13
Nodes (10): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys(), HistoricalBatch, inPlaceholders(), NewNanoID() (+2 more)

### Community 77 - "runRestoreCLI"
Cohesion: 0.27
Nodes (10): cliContext(), loadCLIConfig(), runBackupCLI(), runBackupCommand(), runRestoreCLI(), TestRunBackupCommandRejectsUnknown(), TestRunRestoreCLIRequiresFile(), warnPendingMigrations() (+2 more)

### Community 78 - "TestCov2PureHelpers"
Cohesion: 0.25
Nodes (9): TestSQLiteReadOnlyDSN(), TestSQLiteReadOnlyDSNIsUsable(), prefixCols(), trimSpace(), sqliteDir(), sqliteDSN(), sqlitePath(), SQLiteReadOnlyDSN() (+1 more)

### Community 79 - "main"
Cohesion: 0.29
Nodes (7): TestWantsBackup(), TestWantsBackupAndVersionAreDisjoint(), wantsBackup(), main(), TestVersionDefault(), TestWantsVersion(), wantsVersion()

### Community 80 - "htmx.min.js"
Cohesion: 0.07
Nodes (106): Cutting a release, GitHub Actions, Versioning and release, a(), Ae(), an(), at(), B() (+98 more)

### Community 82 - "Validated"
Cohesion: 0.08
Nodes (20): ApplyOptions, JobState, pending, Report, TableStat, Validated, opener, orDashStr() (+12 more)

### Community 83 - "crypto.go"
Cohesion: 0.12
Nodes (10): openReader, sealWriter, expand(), NewKeyring(), NewNoncePrefix(), NewSalt(), nonceFor(), randomBytes() (+2 more)

### Community 84 - "Configuration"
Cohesion: 0.13
Nodes (15): Accounts and features, Backup and restore, Bank sync (Plaid, optional), Configuration, Core, Currency rates, Database, Email (SMTP) (+7 more)

### Community 85 - "Deployment"
Cohesion: 0.13
Nodes (15): Bind mounts must be owned by 65532, Choosing an engine, Deployment, Docker Compose, First admin, Health check, Mail, Monitoring and logs (+7 more)

### Community 86 - "printDirContents"
Cohesion: 0.40
Nodes (6): printDirContents(), printDirStatus(), captureStdout(), TestPrintDirContentsEmpty(), TestPrintDirContentsListsArchivesAndMarker(), dirStatus

### Community 87 - "Backup and restore"
Cohesion: 0.10
Nodes (20): Archives are secrets, Backup and restore, Check first, Four ways to make a backup, From the admin page, From the command line, Housekeeping, Inspecting an archive (+12 more)

### Community 88 - "rewire_test.go"
Cohesion: 0.09
Nodes (38): pairKey, Row, Shift, Transfer, nameCache, settlementRow, Pair, lessKey() (+30 more)

### Community 91 - "New"
Cohesion: 0.21
Nodes (4): archiveInfo, New(), Server, writeJobJSON()

### Community 93 - "SweepStalePartials"
Cohesion: 0.67
Nodes (3): SweepStalePartials(), TestSweepStalePartials(), TestJanitorIsWiredIntoTheServer()

### Community 94 - "PlaidProvider"
Cohesion: 0.17
Nodes (4): PlaidProvider, FrankfurterProvider, OpenExchangeRatesProvider, swGet()

### Community 99 - "New"
Cohesion: 0.21
Nodes (20): archiveCount(), newBackupSchedService(), TestMaybeBackupNotYetDue(), TestNewWithCronSchedulesNextRun(), TestNewWithInvalidCronDisablesBackupsWithoutPanicking(), TestNewWithoutCronDisablesBackups(), TestScheduledBackupDoesNotBlockTick(), TestScheduledBackupHandlesDumpFailure() (+12 more)

### Community 128 - "prune_test.go"
Cohesion: 0.49
Nodes (9): PruneOldArchives(), lsNames(), TestPruneIgnoresOtherPrefixes(), TestPruneIgnoresUnrelatedFiles(), TestPruneKeepsEverythingWhenDisabled(), TestPruneKeepsNewest(), TestPruneMissingDirIsNotAnError(), TestPruneNoopWhenUnderRetention() (+1 more)

### Community 129 - "Store"
Cohesion: 0.16
Nodes (13): acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), newTestRunnerIn(), Engine, TestOpenMigratesAndOpenNoMigrateDoesNot(), TestOpenNoMigrateLeavesSchemaAlone(), TestPendingMigrationsOnEmptyDatabase() (+5 more)

### Community 130 - "EnsureWritableDir"
Cohesion: 0.22
Nodes (11): EnsureWritableDir(), probeWritable(), mustBePOSIXPermissions(), TestEnsureWritableDirAcceptsWritable(), TestEnsureWritableDirCreatesMissing(), TestEnsureWritableDirNamesEachSetting(), TestEnsureWritableDirRejectsEmpty(), TestEnsureWritableDirReportsUnwritable() (+3 more)

### Community 132 - "web/zz_coverage_test.go"
Cohesion: 0.08
Nodes (27): avatarColor(), categories(), categoryEmoji(), currencyCodes(), firstAlnum(), initials(), methodGlyph(), TestAvatarColorStableAndInRange() (+19 more)

### Community 134 - "Config"
Cohesion: 0.30
Nodes (11): prepareDataDirs(), collectDirStatuses(), pathsCfg(), TestCollectDirStatusesCoversEverySetting(), TestCollectDirStatusesOmitsDatabaseDirForPostgres(), TestCollectDirStatusesReportsUnsetAutoRestore(), TestPrepareDataDirsCreatesEverything(), TestPrepareDataDirsSkipsUnsetAutoRestore() (+3 more)

### Community 135 - "net/http.Request"
Cohesion: 0.11
Nodes (7): Server, safeNext(), Server, fragmentTarget(), Server, isBackgroundRequest(), navSlug()

### Community 137 - "body"
Cohesion: 0.05
Nodes (96): adminOutsiderHarness(), TestAdminActivityRequiresAdmin(), TestAdminCannotAddToForeignGroup(), TestAdminDeleteReturnsToAdminFeed(), TestAdminEditsAnyonesExpenseUntouched(), TestServiceWorkerSkipsPrivateResponses(), TestAdminGroupList(), TestAdminViewsAnyGroupReadOnly() (+88 more)

### Community 141 - "push_test.go"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

## Knowledge Gaps
- **197 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `filterOpt`, `collapseView`, `bankTxRow` (+192 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 299 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `User` connect `User` to `testing.T`, `NewManager`, `newChain`, `atoi64`, `net/http.Request`, `openTestStore`, `Transaction`, `userByEmail`, `ExpenseParticipant`, `Store`, `Group`, `RandomToken`, `Renderer`, `gosplit/main.go`, `.netGroup`?**
  _High betweenness centrality (0.026) - this node is a cross-community bridge._
- **Why does `ctxTimeout()` connect `net/http.ResponseWriter` to `.buildNotificationRows`, `atoi64`, `net/http.Request`, `New`, `Format`, `context.Context`, `gosplit/main.go`, `.handleAdminActivity`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `testing.T` to `gosplit/main.go`, `net/http.Request`?**
  _High betweenness centrality (0.019) - this node is a cross-community bridge._
- **Are the 157 inferred relationships involving `body()` (e.g. with `TestAdminActivityFilters()` and `TestAdminActivityListsEveryoneNeutrally()`) actually correct?**
  _`body()` has 157 INFERRED edges - model-reasoned connections that need verification._
- **Are the 110 inferred relationships involving `newHarness()` (e.g. with `TestAdminActivityRequiresAdmin()` and `TestServiceWorkerSkipsPrivateResponses()`) actually correct?**
  _`newHarness()` has 110 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `filterOpt` to the rest of the system?**
  _197 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.11054421768707483 - nodes in this community are weakly interconnected._