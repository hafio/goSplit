# Graph Report - goSplit  (2026-10-20)

## Corpus Check
- 227 files · ~231,499 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .example 1, .css 1)

## Summary
- 2464 nodes · 10521 edges · 128 communities (87 shown, 41 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1289 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `032f1b94`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- ctxTimeout
- testing.T
- idiomorph-ext.min.js
- NewManager
- Base Layout
- Load
- Server
- .Dump
- openTestStore
- provider_test.go
- ExpenseRecurrence
- configuration.md
- Group
- Format
- Group Detail Page
- crypto_test.go
- nowISO
- DecodeColumn
- database/sql query layer (as-built)
- dev.sh
- render_fragment_test.go
- Store
- currency/zz_coverage_test.go
- schema_test.go
- index.go
- context.Context
- convert.js
- .handleGroupDetail
- Build, Verify, Test Command Policy
- graphify Knowledge Graph Workflow
- balance_view (derived balances)
- dev.sh / dev.ps1 task runners
- split.go
- Auth (magic-link + password)
- SQLite default (no DB container)
- mail_test.go
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
- roundtrip_test.go
- NewUUID
- Development
- NewJobTracker
- seedDirectExpense
- sw.js
- Architecture
- .buildNotificationRows
- web/theme_test.go
- recovery_test.go
- BackupSchedule
- Client behaviour
- Notification
- countOf
- userByEmail
- parseExpenseInput
- .buildBatches
- nameCache
- settledChainHarness
- httpapp/zz_coverage3_test.go
- htmx.min.js
- github.com/hafio/gosplit
- Validated
- crypto.go
- Configuration
- Deployment
- filesOnly
- Backup and restore
- rewire_test.go
- NoneProvider
- net/http.ResponseWriter
- PlaidProvider
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
- PruneOldArchives
- Store
- EnsureWritableDir
- web/zz_coverage_test.go
- Config
- net/http.Request
- body
- push_test.go
- ownerOf

## God Nodes (most connected - your core abstractions)
1. `body()` - 176 edges
2. `newHarness()` - 131 edges
3. `newTestService()` - 91 edges
4. `ctxTimeout()` - 70 edges
5. `Expense` - 63 edges
6. `User` - 60 edges
7. `covUser()` - 55 edges
8. `openTestStore()` - 52 edges
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
- `loadCLIConfig()` --references--> `Config`  [EXTRACTED]
  cmd/gosplit/backup_cli.go → internal/config/config.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Templates Composed Into Base Layout** — internal_web_templates_layout_base_layout, internal_web_templates_groups_page, internal_web_templates_import_page, internal_web_templates_login_page, internal_web_templates_message_page, internal_web_templates_profile_page, internal_web_templates_recurring_page, internal_web_templates_register_page, internal_web_templates_reset_page [EXTRACTED 1.00]
- **Friend balance and settlement flow** — internal_web_templates_friend_page, internal_web_templates_convert_page, internal_web_templates_balances_page, internal_web_templates_route_friend_convert, internal_web_templates_route_friend_settle [INFERRED 0.75]
- **Authentication Form Screens** — internal_web_templates_login_page, internal_web_templates_register_page, internal_web_templates_reset_page [INFERRED 0.85]
- **Base layout content-block pattern** — internal_web_templates_activity_page, internal_web_templates_admin_page, internal_web_templates_balances_page, internal_web_templates_bank_page, internal_web_templates_convert_page, internal_web_templates_expense_detail_page, internal_web_templates_expense_form_page, internal_web_templates_forgot_page, internal_web_templates_friend_page, internal_web_templates_friends_page, internal_web_templates_group_page [INFERRED 0.85]
- **Expense lifecycle flow** — internal_web_templates_expense_form_page, internal_web_templates_expense_detail_page, internal_web_templates_route_expense_new, internal_web_templates_route_expense_move, internal_web_templates_route_expense_delete [INFERRED 0.85]

## Communities (128 total, 41 thin omitted)

### Community 0 - "ctxTimeout"
Cohesion: 0.11
Nodes (7): Server, applySubmittedFields(), Server, atoi64(), ctxTimeout(), ValidRole(), TestCovModelFlags()

### Community 1 - "testing.T"
Cohesion: 0.04
Nodes (121): nullInt(), TestIsAdminEmail(), TestJanitorIsWiredIntoTheServer(), TestJanitorPrunesAbandonedUpload(), TestStartJanitorStopsWithContext(), newOverrideFixture(), TestAdminEditsAndDeletesAnyTransaction(), TestAdminGetsNoGroupLevelPowers() (+113 more)

### Community 2 - "idiomorph-ext.min.js"
Cohesion: 0.28
Nodes (15): a(), c(), d(), e(), f(), h(), i(), l() (+7 more)

### Community 3 - "NewManager"
Cohesion: 0.10
Nodes (33): CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler(), covOpenStore(), TestCovAuthenticate_ExistingCSRFReused() (+25 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.07
Nodes (33): sanitizeErr(), TestSanitizeErr(), TestSanitizeErrTruncates(), backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults() (+25 more)

### Community 6 - "Server"
Cohesion: 0.16
Nodes (6): candidate, displayName(), Server, prefillFromForm(), markPrivate(), TestPrefillFromForm()

### Community 7 - ".Dump"
Cohesion: 0.21
Nodes (7): queryer, Runner, readMigrationVersions(), writeTarFile(), SweepStalePartials(), TestSweepStalePartials(), Server

### Community 8 - "openTestStore"
Cohesion: 0.06
Nodes (57): amountPtr(), expenseNames(), nullGroup(), TestListAllActivity(), TestListAllGroupsAndCounts(), TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestUserNetByExpense() (+49 more)

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
Cohesion: 0.16
Nodes (7): groupListRow, hasGroup(), Store, scanGroup(), Group, orDefault(), overrideFixture

### Community 13 - "Format"
Cohesion: 0.17
Nodes (12): absInt64(), Server, friendSettlePrefill(), groupSettleSuggestion(), decimals(), Format(), Parse(), pow10() (+4 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "crypto_test.go"
Cohesion: 0.21
Nodes (22): NewSealWriter(), frameAt(), seal(), TestDuplicatedFrameFails(), TestEmptyPayloadStillTerminates(), TestFingerprintDistinguishesSecrets(), TestFingerprintDoesNotRevealSealKey(), TestFingerprintIsStable() (+14 more)

### Community 16 - "nowISO"
Cohesion: 0.10
Nodes (10): Store, FromNow(), NewNanoID(), nowISO(), Store, Store, Store, TestCovIDGenerators() (+2 more)

### Community 17 - "DecodeColumn"
Cohesion: 0.11
Nodes (22): Column, ColumnKind, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), ScanDest(), nullBool() (+14 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "render_fragment_test.go"
Cohesion: 0.09
Nodes (29): TestNotificationKindsHaveLocaleEntries(), Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load(), TestDetect() (+21 more)

### Community 21 - "Store"
Cohesion: 0.19
Nodes (6): BuildLikePattern(), ExpenseFilter, Store, scanExpense(), TestBuildLikePattern(), GroupScope

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.13
Nodes (23): covRoundTripFunc, FrankfurterProvider, OpenExchangeRatesProvider, defaultClient(), covClient(), covJSONResp(), TestFrankfurterRateMalformedJSON(), TestFrankfurterRateMissingRate() (+15 more)

### Community 23 - "schema_test.go"
Cohesion: 0.10
Nodes (26): batcher, Report, Table, orDashStr(), writeMarker(), SameMigrationSet(), snapshotTables(), DeleteOrder() (+18 more)

### Community 24 - "index.go"
Cohesion: 0.14
Nodes (22): countingReader, Entry, EntryKind, Index, Limits, Opener, opener, decodeManifest() (+14 more)

### Community 25 - "context.Context"
Cohesion: 0.07
Nodes (15): itoa(), Service, Service, Service, Store, Store, User, Store (+7 more)

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 27 - ".handleGroupDetail"
Cohesion: 0.12
Nodes (14): currencyNet, expenseRow, groupRow, monthGroup, TestGroupByMonth(), Server, applyFeedLimit(), dateOnly() (+6 more)

### Community 32 - "split.go"
Cohesion: 0.08
Nodes (43): formatValue(), lineForMethod(), orZero(), prefillStored(), rebalanceToTarget(), sumTarget(), valueForMethod(), TestPrefillStoredKeepsCompleteSet() (+35 more)

### Community 35 - "mail_test.go"
Cohesion: 0.13
Nodes (10): buildMessage(), New(), serveFakeSMTP(), TestBuildMessage(), TestLogMailerSend(), TestNewLogMailerWhenSMTPUnset(), TestNewSMTPMailerWhenHostSet(), TestSMTPMailerDialError() (+2 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.10
Nodes (20): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+12 more)

### Community 39 - "Expense"
Cohesion: 0.12
Nodes (20): expenseDetailView, participantView, prefillDerived(), prefillEdit(), buildHistoricalCSV(), dateOnly(), ExpenseInput, Service (+12 more)

### Community 40 - "Transaction"
Cohesion: 0.10
Nodes (6): Disabled, Provider, Transaction, Service, bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

### Community 42 - "index_test.go"
Cohesion: 0.27
Nodes (26): craftEntry, ReadHeader(), tablePartName(), b64(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive() (+18 more)

### Community 43 - "tripHarness"
Cohesion: 0.09
Nodes (40): adminOutsiderHarness(), expenseByName(), TestAdminActivityFilters(), TestAdminActivityListsEveryoneNeutrally(), TestAdminActivityShowAllKeepsFilters(), TestAdminCannotAddToForeignGroup(), TestAdminDeleteReturnsToAdminFeed(), TestAdminEditsAnyonesExpenseUntouched() (+32 more)

### Community 44 - "Store"
Cohesion: 0.18
Nodes (7): friendRow, CumulatedBalance, Store, scanBalances(), Store, Balance, PlannedExpense

### Community 45 - "handlers_backup_test.go"
Cohesion: 0.11
Nodes (25): adminHarness(), harness, TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted(), TestBackupDownloadRejectsHostileNames(), TestBackupDownloadServesArchive(), TestBackupGenerateAndDownloadRoundTrip(), TestBackupJobStatusUnknownToken() (+17 more)

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
Cohesion: 0.43
Nodes (4): Header, decodeB64(), WriteHeader(), InspectFile()

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
Cohesion: 0.14
Nodes (8): harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders(), TestStaticCacheHeaders(), covPostJSON()

### Community 56 - "gosplit/main.go"
Cohesion: 0.16
Nodes (15): ctxKey, balanceRow, bankTxRow, collapseView, filterOpt, recurrenceRow, templateOption, TestAssetURLFingerprinted() (+7 more)

### Community 57 - "backup_cli_test.go"
Cohesion: 0.09
Nodes (33): cliContext(), humanBytes(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI() (+25 more)

### Community 58 - "bank.js"
Cohesion: 0.83
Nodes (3): connect(), csrf(), wire()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 60 - "roundtrip_test.go"
Cohesion: 0.29
Nodes (19): countRows(), exec(), newTestRunner(), seedEverything(), seedUploads(), wipeAll(), TestDumpToDirFailsFastOnUnwritableDir(), TestDumpIsDeterministic() (+11 more)

### Community 61 - "NewUUID"
Cohesion: 0.15
Nodes (6): Service, nullStr(), Service, Service, nullableString(), NewUUID()

### Community 62 - "Development"
Cohesion: 0.13
Nodes (15): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Dev scripts, Development (+7 more)

### Community 63 - "NewJobTracker"
Cohesion: 0.27
Nodes (10): NewJobTracker(), TestDiscardPendingReleases(), TestJobTrackerLifecycle(), TestJobTrackerRecordsFailure(), TestJobTrackerUnknownToken(), TestPruneDropsExpiredJobsAndPending(), TestPruneKeepsRunningJobs(), TestStartRestoreMarksTheJob() (+2 more)

### Community 64 - "seedDirectExpense"
Cohesion: 0.15
Nodes (22): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+14 more)

### Community 66 - "Architecture"
Cohesion: 0.11
Nodes (19): Architecture, Background jobs, Backup archives, Balances are derived, never stored, Collapsing history, Data model, Decisions worth keeping, Internationalisation (+11 more)

### Community 67 - ".buildNotificationRows"
Cohesion: 0.18
Nodes (8): notificationRow, TestFeedDateParts(), feedDateParts(), entityHref(), Server, TestNotificationEntityHrefFallback(), FormatWithCode(), TestFormatWithCode()

### Community 68 - "web/theme_test.go"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 69 - "recovery_test.go"
Cohesion: 0.37
Nodes (14): dirExists(), RecoverIncompleteSwap(), marker(), swapFixture(), TestRecoverCleanupOnlyCrash(), TestRecoverIsIdempotent(), TestRecoverMidSwapCrash(), TestRecoverMidSwapCrashWithoutStaging() (+6 more)

### Community 70 - "BackupSchedule"
Cohesion: 0.33
Nodes (4): TestBackupScheduleParserMatchesRecurrences(), TestSameDir(), BackupSchedule(), sameDir()

### Community 71 - "Client behaviour"
Cohesion: 0.29
Nodes (7): Client behaviour, Fragments, Freshness, Measurement, Navigation, Progressive web app, Safeguards

### Community 72 - "Notification"
Cohesion: 0.17
Nodes (6): Service, notificationsFor(), TestNotificationsForSkipsActorAndEmpty(), Notification, Store, scanNotification()

### Community 73 - "countOf"
Cohesion: 0.42
Nodes (7): TestAdminViewsAnyGroupReadOnly(), countOf(), groupFlags(), outsiderHarness(), TestGroupActionsRejectOutsider(), TestGroupMembersHiddenFromOutsider(), TestTidyUpClearsLeftoverPairs()

### Community 74 - "userByEmail"
Cohesion: 0.25
Nodes (13): limitedHarness(), TestLimitedCannotCreateGroup(), TestLimitedPagesHideGroupCreationAndImport(), reread(), seedNotification(), TestBellBadgeAppearsOnEveryPage(), TestNotificationBadgeAndMenuFragments(), TestNotificationOpenIsScopedToItsOwner() (+5 more)

### Community 75 - "parseExpenseInput"
Cohesion: 0.20
Nodes (9): fieldError, fieldErrorf(), fieldErrorsFor(), formVersion(), parseExpenseInput(), parseSettlementInput(), TestFieldErrorsFor(), TestFormVersionFailsClosed() (+1 more)

### Community 76 - ".buildBatches"
Cohesion: 0.17
Nodes (9): Service, sortedInt64Keys(), sortedStrKeys(), HistoricalBatch, inPlaceholders(), prefixCols(), trimSpace(), sqliteDir() (+1 more)

### Community 78 - "settledChainHarness"
Cohesion: 0.60
Nodes (4): nettingBetween(), settledChainHarness(), TestNettingEntryRefusesChanges(), TestNettingRowsRenderReadOnly()

### Community 79 - "httpapp/zz_coverage3_test.go"
Cohesion: 0.60
Nodes (3): covSeedBankTx(), TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 80 - "htmx.min.js"
Cohesion: 0.07
Nodes (106): Cutting a release, GitHub Actions, Versioning and release, a(), Ae(), an(), at(), B() (+98 more)

### Community 82 - "Validated"
Cohesion: 0.11
Nodes (12): ApplyOptions, JobState, pending, TableStat, Validated, Manifest, JobStatus, JobTracker (+4 more)

### Community 83 - "crypto.go"
Cohesion: 0.10
Nodes (14): Keyring, openReader, sealWriter, expand(), newAEAD(), NewKeyring(), NewNoncePrefix(), NewOpenReader() (+6 more)

### Community 84 - "Configuration"
Cohesion: 0.13
Nodes (15): Accounts and features, Backup and restore, Bank sync (Plaid, optional), Configuration, Core, Currency rates, Database, Email (SMTP) (+7 more)

### Community 85 - "Deployment"
Cohesion: 0.13
Nodes (15): Bind mounts must be owned by 65532, Choosing an engine, Deployment, Docker Compose, First admin, Health check, Mail, Monitoring and logs (+7 more)

### Community 87 - "Backup and restore"
Cohesion: 0.10
Nodes (20): Archives are secrets, Backup and restore, Check first, Four ways to make a backup, From the admin page, From the command line, Housekeeping, Inspecting an archive (+12 more)

### Community 88 - "rewire_test.go"
Cohesion: 0.08
Nodes (50): pairKey, Row, Shift, Transfer, settlementRow, Pair, lessKey(), NeedsTidy() (+42 more)

### Community 90 - "NoneProvider"
Cohesion: 0.17
Nodes (3): NoneProvider, covErrRates, fakeRates

### Community 91 - "net/http.ResponseWriter"
Cohesion: 0.12
Nodes (7): archiveInfo, New(), Server, Server, writeJobJSON(), Server, Server

### Community 99 - "New"
Cohesion: 0.10
Nodes (30): parseLogLevel(), run(), Provider, NewProvider(), TestNewProviderSelection(), New(), Mailer, archiveCount() (+22 more)

### Community 128 - "PruneOldArchives"
Cohesion: 0.39
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
Cohesion: 0.19
Nodes (16): prepareDataDirs(), collectDirStatuses(), pathsCfg(), TestCollectDirStatusesCoversEverySetting(), TestCollectDirStatusesOmitsDatabaseDirForPostgres(), TestCollectDirStatusesReportsUnsetAutoRestore(), TestPrepareDataDirsCreatesEverything(), TestPrepareDataDirsSkipsUnsetAutoRestore() (+8 more)

### Community 135 - "net/http.Request"
Cohesion: 0.13
Nodes (7): Server, safeNext(), TestSafeNextRefusesOffSiteTargets(), fragmentTarget(), Server, isBackgroundRequest(), navSlug()

### Community 137 - "body"
Cohesion: 0.05
Nodes (97): TestAdminActivityRequiresAdmin(), TestServiceWorkerSkipsPrivateResponses(), TestMemberAdminKeepsGroupControls(), TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse() (+89 more)

### Community 141 - "push_test.go"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

## Knowledge Gaps
- **197 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `filterOpt`, `collapseView`, `bankTxRow` (+192 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 299 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **41 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ctxTimeout()` connect `ctxTimeout` to `.buildNotificationRows`, `Server`, `net/http.Request`, `net/http.ResponseWriter`, `parseExpenseInput`, `Format`, `gosplit/main.go`, `context.Context`, `.handleGroupDetail`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `testing.T` to `gosplit/main.go`, `net/http.Request`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **Why does `User` connect `context.Context` to `ctxTimeout`, `testing.T`, `NewManager`, `Server`, `net/http.Request`, `openTestStore`, `Transaction`, `userByEmail`, `Notification`, `Store`, `Group`, `RandomToken`, `Renderer`, `gosplit/main.go`, `rewire_test.go`, `NewUUID`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **Are the 166 inferred relationships involving `body()` (e.g. with `TestAdminActivityFilters()` and `TestAdminActivityListsEveryoneNeutrally()`) actually correct?**
  _`body()` has 166 INFERRED edges - model-reasoned connections that need verification._
- **Are the 118 inferred relationships involving `newHarness()` (e.g. with `TestAdminActivityRequiresAdmin()` and `TestServiceWorkerSkipsPrivateResponses()`) actually correct?**
  _`newHarness()` has 118 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `filterOpt` to the rest of the system?**
  _197 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `ctxTimeout` be split into smaller, more focused modules?**
  _Cohesion score 0.1076923076923077 - nodes in this community are weakly interconnected._