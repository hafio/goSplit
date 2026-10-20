# Graph Report - goSplit  (2026-10-20)

## Corpus Check
- 234 files · ~243,583 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .example 1, .css 1)

## Summary
- 2549 nodes · 11467 edges · 139 communities (93 shown, 46 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1419 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `33c81135`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- ctxTimeout
- newTestService
- idiomorph-ext.min.js
- NewManager
- Base Layout
- testing.T
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
- nowISO
- DecodeColumn
- database/sql query layer (as-built)
- dev.sh
- web/zz_coverage_test.go
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
- Method
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
- adminHarness
- RandomToken
- covfilter/main.go
- Header
- GoSplit UI Icon Sprite Sheet
- push.js
- Golden Scenario regression fixture
- countRows
- Renderer
- harness
- gosplit/main.go
- Config
- bank.js
- GoSplit App Icon
- newTestRunner
- .netGroup
- Development
- net/http.Request
- httpapp/split_inputs_test.go
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
- captureStdout
- Compute
- covPair
- settledChainHarness
- New
- htmx.min.js
- github.com/hafio/gosplit
- Validated
- crypto.go
- Configuration
- Deployment
- filesOnly
- Backup and restore
- rewire_test.go
- extractToken
- NoneProvider
- net/http.ResponseWriter
- run
- countingRates
- PlaidProvider
- i18n.go
- fakeRates
- nullInt
- EnsureWritableDir
- Service
- .moveFormData
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
- prefillStored
- i18n_test.go
- Store
- newOverrideFixture
- nameCache
- httpapp/zz_coverage3_test.go
- Sender
- pathsCfg
- .collapse
- body
- push/push_test.go
- ownerOf

## God Nodes (most connected - your core abstractions)
1. `body()` - 186 edges
2. `newHarness()` - 133 edges
3. `newTestService()` - 98 edges
4. `ctxTimeout()` - 70 edges
5. `Expense` - 63 edges
6. `User` - 60 edges
7. `covUser()` - 59 edges
8. `openTestStore()` - 57 edges
9. `Config` - 42 edges
10. `Store` - 40 edges

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

## Communities (139 total, 46 thin omitted)

### Community 0 - "ctxTimeout"
Cohesion: 0.13
Nodes (4): Server, Server, sortedNets(), ctxTimeout()

### Community 1 - "newTestService"
Cohesion: 0.07
Nodes (52): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestArchiveNothing(), TestCreateConversionExact(), TestCreateConversionExactDatesABlankDateToday(), TestDeleteExpenseRejectsStaleVersion(), TestAddFriendToGroup(), TestToggleHiddenFriendKeepsTheOthers() (+44 more)

### Community 2 - "idiomorph-ext.min.js"
Cohesion: 0.28
Nodes (15): a(), c(), d(), e(), f(), h(), i(), l() (+7 more)

### Community 3 - "NewManager"
Cohesion: 0.10
Nodes (33): CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler(), covOpenStore(), TestCovAuthenticate_ExistingCSRFReused() (+25 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "testing.T"
Cohesion: 0.05
Nodes (77): inspectDir(), probeDir(), TestInspectDirEmptyPath(), TestInspectDirMissing(), TestInspectDirMissingUnderAnUnwritableParent(), TestInspectDirNotADirectory(), TestInspectDirUnwritable(), TestInspectDirWritable() (+69 more)

### Community 6 - "atoi64"
Cohesion: 0.10
Nodes (16): candidate, fieldError, applySubmittedFields(), displayName(), fieldErrorf(), fieldErrorsFor(), formVersion(), Server (+8 more)

### Community 7 - ".Dump"
Cohesion: 0.18
Nodes (9): queryer, b64(), Runner, readMigrationVersions(), writeTarFile(), TestHeaderRejectsAbsurdKDFParameters(), SweepStalePartials(), TestSweepStalePartials() (+1 more)

### Community 8 - "openTestStore"
Cohesion: 0.06
Nodes (58): amountPtr(), expenseNames(), nullGroup(), TestListAllActivity(), TestListAllGroupsAndCounts(), TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestUserNetByExpense() (+50 more)

### Community 9 - "provider_test.go"
Cohesion: 0.26
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.16
Nodes (8): Service, Service, toISO(), todayISO(), TestGetRateWithoutADateRefreshesDaily(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "configuration.md"
Cohesion: 0.41
Nodes (6): app service, splitpro-data volume, All pages, GoSplit documentation, Start here, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.11
Nodes (15): currencyNet, groupListRow, groupRow, hasGroup(), inPlaceholders(), prefixCols(), trimSpace(), Store (+7 more)

### Community 13 - "Format"
Cohesion: 0.16
Nodes (13): absInt64(), Server, friendSettlePrefill(), groupSettleSuggestion(), allDigits(), decimals(), Format(), Parse() (+5 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "crypto_test.go"
Cohesion: 0.18
Nodes (20): frameAt(), open(), seal(), TestDuplicatedFrameFails(), TestEmptyPayloadStillTerminates(), TestFingerprintDistinguishesSecrets(), TestFingerprintDoesNotRevealSealKey(), TestFingerprintIsStable() (+12 more)

### Community 16 - "nowISO"
Cohesion: 0.11
Nodes (10): acquireWithRenewal(), Store, FromNow(), nowISO(), Store, Store, Store, TestCovRecurrences() (+2 more)

### Community 17 - "DecodeColumn"
Cohesion: 0.14
Nodes (16): Column, ColumnKind, DecodeColumn(), isJSONNull(), isJSONSpace(), ScanDest(), TestDecodeBoolBindsPerEngine(), TestDecodeColumnRejectsBareNumber() (+8 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "web/zz_coverage_test.go"
Cohesion: 0.05
Nodes (46): avatarColor(), categories(), categoryEmoji(), currencyCodes(), firstAlnum(), initials(), methodGlyph(), TestAssetURLFingerprinted() (+38 more)

### Community 21 - "Store"
Cohesion: 0.17
Nodes (5): BuildLikePattern(), ExpenseFilter, Store, TestBuildLikePattern(), GroupScope

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.24
Nodes (19): covRoundTripFunc, covClient(), covJSONResp(), TestFrankfurterRateMalformedJSON(), TestFrankfurterRateMissingRate(), TestFrankfurterRateStatusError(), TestFrankfurterRateSuccess(), TestFrankfurterRateTransportError() (+11 more)

### Community 23 - "schema_test.go"
Cohesion: 0.13
Nodes (20): batcher, Table, DeleteOrder(), InsertOrder(), isBareIdent(), quoteIdent(), quoteIdents(), TableCount() (+12 more)

### Community 24 - "index.go"
Cohesion: 0.14
Nodes (21): countingReader, Entry, EntryKind, Index, Limits, Opener, decodeRows(), drainCapped() (+13 more)

### Community 25 - "context.Context"
Cohesion: 0.07
Nodes (15): Service, Service, Service, Store, Store, User, Store, Store (+7 more)

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 27 - ".handleGroupDetail"
Cohesion: 0.28
Nodes (8): expenseRow, monthGroup, TestGroupByMonth(), applyFeedLimit(), groupByMonth(), monthLabel(), parseFilter(), showAllHref()

### Community 32 - "Method"
Cohesion: 0.11
Nodes (28): formatValue(), lineForMethod(), orZero(), valueForMethod(), TestValueFormatRoundTrip(), TestValueForMethodEqualAndUnknown(), TestValueForMethodErrors(), generatedFrom() (+20 more)

### Community 35 - "mail_test.go"
Cohesion: 0.11
Nodes (12): buildMessage(), New(), serveFakeSMTP(), startTLSRelay(), TestBuildMessage(), TestLogMailerSend(), TestNewLogMailerWhenSMTPUnset(), TestNewSMTPMailerWhenHostSet() (+4 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.10
Nodes (20): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+12 more)

### Community 39 - "Expense"
Cohesion: 0.08
Nodes (31): expenseDetailView, participantView, prefillDerived(), prefillEdit(), buildHistoricalCSV(), dateOnly(), sortedInt64Keys(), sortedStrKeys() (+23 more)

### Community 40 - "Transaction"
Cohesion: 0.10
Nodes (6): Disabled, Provider, Transaction, Service, bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

### Community 42 - "index_test.go"
Cohesion: 0.30
Nodes (28): craftEntry, ReadHeader(), tablePartName(), testKeyring(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive() (+20 more)

### Community 43 - "tripHarness"
Cohesion: 0.09
Nodes (41): expenseByName(), TestAdminActivityFilters(), TestAdminActivityListsEveryoneNeutrally(), TestAdminActivityShowAllKeepsFilters(), TestAdminEditsDirectExpenseUntouched(), TestBuildRowsViewerZeroIsNeutral(), TestDeleteStoreFailureIsReported(), TestEditFormKeepsArchivedCurrentGroup() (+33 more)

### Community 44 - "Store"
Cohesion: 0.20
Nodes (6): friendRow, CumulatedBalance, Store, scanBalances(), Store, Balance

### Community 45 - "adminHarness"
Cohesion: 0.10
Nodes (24): adminHarness(), harness, tableCounts(), TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted(), TestBackupDownloadRejectsHostileNames(), TestBackupDownloadServesArchive(), TestBackupGenerateAndDownloadRoundTrip() (+16 more)

### Community 46 - "RandomToken"
Cohesion: 0.13
Nodes (11): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+3 more)

### Community 48 - "covfilter/main.go"
Cohesion: 0.12
Nodes (26): block, marker, pos, span, unit, applyMarkers(), directive(), main() (+18 more)

### Community 49 - "Header"
Cohesion: 0.20
Nodes (10): Header, orDash(), runInspectCLI(), TestInspectRejectsNonArchive(), TestOrDash(), TestRunInspectCLIReportsMissingFile(), TestRunInspectCLIRequiresExactlyOnePath(), decodeB64() (+2 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.57
Nodes (6): b64ToUint8(), csrf(), enable(), test(), wire(), wireOnce()

### Community 53 - "countRows"
Cohesion: 0.14
Nodes (17): TestIsAdminFailsClosed(), countRows(), mkExp(), netByUser(), TestArchiveCSVNoteFormat(), TestArchiveDirectPreservesNet(), TestArchiveGroupPreservesNet(), TestArchiveMultiCurrency() (+9 more)

### Community 54 - "Renderer"
Cohesion: 0.26
Nodes (4): Renderer, ViewData, htmlHeaders(), setServerTiming()

### Community 55 - "harness"
Cohesion: 0.15
Nodes (8): TestBackupRoutesRejectAnonymous(), harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders(), TestStaticCacheHeaders()

### Community 56 - "gosplit/main.go"
Cohesion: 0.20
Nodes (11): ctxKey, balanceRow, bankTxRow, collapseView, filterOpt, recurrenceRow, templateOption, friendExport (+3 more)

### Community 57 - "Config"
Cohesion: 0.16
Nodes (28): cliContext(), loadCLIConfig(), printManifest(), runBackupCLI(), runBackupCommand(), runRestoreCLI(), addUser(), archivesIn() (+20 more)

### Community 58 - "bank.js"
Cohesion: 0.83
Nodes (3): connect(), csrf(), wire()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 60 - "newTestRunner"
Cohesion: 0.15
Nodes (38): CheckAutoRestore(), findAutoRestoreArchive(), addCarol(), autoRestoreFixture(), mustGlob(), safetyDumps(), TestCheckAutoRestoreFailuresAbortTheBoot(), TestCheckAutoRestoreLeavesItToTheLockHolder() (+30 more)

### Community 61 - ".netGroup"
Cohesion: 0.24
Nodes (4): nullStr(), Service, Service, nullableString()

### Community 62 - "Development"
Cohesion: 0.13
Nodes (15): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Dev scripts, Development (+7 more)

### Community 63 - "net/http.Request"
Cohesion: 0.14
Nodes (6): Server, safeNext(), fragmentTarget(), Server, isBackgroundRequest(), navSlug()

### Community 64 - "httpapp/split_inputs_test.go"
Cohesion: 0.15
Nodes (23): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+15 more)

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
Cohesion: 0.21
Nodes (4): Service, Notification, Store, scanNotification()

### Community 73 - "countOf"
Cohesion: 0.16
Nodes (15): adminOutsiderHarness(), TestAdminCannotAddToForeignGroup(), TestAdminDeleteReturnsToAdminFeed(), TestAdminEditsAnyonesExpenseUntouched(), TestAdminGroupList(), TestAdminViewsAnyGroupReadOnly(), TestMemberAdminKeepsGroupControls(), TestDeletingAConversionLegDeletesBoth() (+7 more)

### Community 74 - "userByEmail"
Cohesion: 0.21
Nodes (15): limitedHarness(), TestAdminConsoleLimitedRole(), TestLimitedCannotCreateGroup(), TestLimitedPagesHideGroupCreationAndImport(), reread(), seedNotification(), TestBellBadgeAppearsOnEveryPage(), TestNotificationBadgeAndMenuFragments() (+7 more)

### Community 75 - "captureStdout"
Cohesion: 0.20
Nodes (10): humanBytes(), TestHumanBytes(), printDirContents(), printDirStatus(), capture(), captureStdout(), TestPrintDirContentsEmpty(), TestPrintDirContentsListsArchivesAndMarker() (+2 more)

### Community 76 - "Compute"
Cohesion: 0.16
Nodes (18): TestSplitRemainderStableAcrossLineOrder(), Compute(), computeShares(), distributeRemainder(), finalize(), gcd(), seededOrder(), splitmix64() (+10 more)

### Community 77 - "covPair"
Cohesion: 0.17
Nodes (20): covInput(), covNotif(), covNotifications(), covOptInEmail(), covPair(), covUserMap(), TestNotifyAddExpenseRecordsForOthersOnly(), TestNotifyDeletedExpenseKeepsItsNotification() (+12 more)

### Community 78 - "settledChainHarness"
Cohesion: 0.60
Nodes (4): nettingBetween(), settledChainHarness(), TestNettingEntryRefusesChanges(), TestNettingRowsRenderReadOnly()

### Community 79 - "New"
Cohesion: 0.30
Nodes (15): archiveCount(), newBackupSchedService(), TestMaybeBackupNotYetDue(), TestNewWithCronSchedulesNextRun(), TestNewWithInvalidCronDisablesBackupsWithoutPanicking(), TestNewWithoutCronDisablesBackups(), TestScheduledBackupDoesNotBlockTick(), TestScheduledBackupHandlesDumpFailure() (+7 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.07
Nodes (106): Cutting a release, GitHub Actions, Versioning and release, a(), Ae(), an(), at(), B() (+98 more)

### Community 82 - "Validated"
Cohesion: 0.08
Nodes (19): ApplyOptions, JobState, pending, Report, TableStat, Validated, opener, orDashStr() (+11 more)

### Community 83 - "crypto.go"
Cohesion: 0.10
Nodes (15): Keyring, openReader, sealWriter, expand(), newAEAD(), NewKeyring(), NewNoncePrefix(), NewOpenReader() (+7 more)

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
Nodes (51): pairKey, Row, Shift, Transfer, settlementRow, Pair, lessKey(), NeedsTidy() (+43 more)

### Community 89 - "extractToken"
Cohesion: 0.24
Nodes (8): extractToken(), TestAdminAutoPromotionNeedsAVerifiedAddress(), TestForgotResetPassword(), TestMagicLinkFlow(), TestCovAuthErrorBranches(), TestCovAuthInactiveAndPromotion(), TestAdminMagicLinkForUser(), fakeMailer

### Community 91 - "net/http.ResponseWriter"
Cohesion: 0.10
Nodes (7): archiveInfo, Server, Server, writeJobJSON(), markPrivate(), Server, Server

### Community 92 - "run"
Cohesion: 0.18
Nodes (10): TestWantsBackup(), TestWantsBackupAndVersionAreDisjoint(), wantsBackup(), main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion() (+2 more)

### Community 94 - "PlaidProvider"
Cohesion: 0.17
Nodes (4): PlaidProvider, FrankfurterProvider, OpenExchangeRatesProvider, swGet()

### Community 95 - "i18n.go"
Cohesion: 0.23
Nodes (6): TestNotificationKindsHaveLocaleEntries(), Bundle, Load(), parseAcceptLanguage(), parseQ(), TestTemplateKeysResolve()

### Community 97 - "nullInt"
Cohesion: 0.23
Nodes (11): EncodeColumn(), nullBool(), nullInt(), nullString(), TestDecodeColumnRoundTrip(), TestEncodeColumn(), TestEncodeColumnRejectsUnexpectedNull(), TestDeleteExpenseAuthorization() (+3 more)

### Community 98 - "EnsureWritableDir"
Cohesion: 0.22
Nodes (11): EnsureWritableDir(), probeWritable(), mustBePOSIXPermissions(), TestEnsureWritableDirAcceptsWritable(), TestEnsureWritableDirCreatesMissing(), TestEnsureWritableDirNamesEachSetting(), TestEnsureWritableDirRejectsEmpty(), TestEnsureWritableDirReportsUnwritable() (+3 more)

### Community 99 - "Service"
Cohesion: 0.13
Nodes (12): defaultClient(), Provider, NewProvider(), TestNewProviderSelection(), Mailer, newSchedService(), TestNew(), TestRunStopsOnCancel() (+4 more)

### Community 100 - ".moveFormData"
Cohesion: 0.25
Nodes (3): Server, dateOnly(), Server

### Community 127 - "prefillStored"
Cohesion: 0.25
Nodes (8): prefillStored(), rebalanceToTarget(), sumTarget(), TestPrefillStoredKeepsCompleteSet(), TestPrefillStoredRebalancesDepartedParticipant(), TestRebalanceToTargetEdges(), TestRebalanceToTargetPicksTheLargest(), TestSumTargetPerMethod()

### Community 128 - "i18n_test.go"
Cohesion: 0.43
Nodes (7): firstOther(), load(), TestDetect(), TestLanguages(), TestNoOrphanTranslationKeys(), TestTranslate(), TestTranslatedKeysInEveryLocale()

### Community 129 - "Store"
Cohesion: 0.15
Nodes (13): warnPendingMigrations(), newTestRunnerIn(), Engine, TestSQLiteReadOnlyDSN(), TestSQLiteReadOnlyDSNIsUsable(), connect(), Store, Open() (+5 more)

### Community 130 - "newOverrideFixture"
Cohesion: 0.32
Nodes (6): newOverrideFixture(), TestAdminEditsAndDeletesAnyTransaction(), TestAdminGetsNoGroupLevelPowers(), TestAdminMoveSkipsOnlyTheActorCheck(), TestAdminSettlementEditSkipsOnlyTheActorCheck(), overrideFixture

### Community 132 - "httpapp/zz_coverage3_test.go"
Cohesion: 0.60
Nodes (3): covSeedBankTx(), TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 134 - "pathsCfg"
Cohesion: 0.26
Nodes (13): prepareBoot(), prepareDataDirs(), collectDirStatuses(), pathsCfg(), TestCollectDirStatusesCoversEverySetting(), TestCollectDirStatusesOmitsDatabaseDirForPostgres(), TestCollectDirStatusesReportsUnsetAutoRestore(), TestPrepareBootCreatesTheDirectories() (+5 more)

### Community 137 - "body"
Cohesion: 0.05
Nodes (98): TestAdminActivityRequiresAdmin(), TestServiceWorkerSkipsPrivateResponses(), TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection() (+90 more)

### Community 141 - "push/push_test.go"
Cohesion: 0.26
Nodes (11): New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDeliversAndReportsGone(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+3 more)

## Knowledge Gaps
- **196 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `filterOpt`, `collapseView`, `bankTxRow` (+191 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 301 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **46 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ctxTimeout()` connect `ctxTimeout` to `.buildNotificationRows`, `.moveFormData`, `atoi64`, `.handleGroupDetail`, `Format`, `gosplit/main.go`, `context.Context`, `net/http.ResponseWriter`, `net/http.Request`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `testing.T` to `gosplit/main.go`, `net/http.Request`?**
  _High betweenness centrality (0.026) - this node is a cross-community bridge._
- **Why does `User` connect `context.Context` to `newTestService`, `newOverrideFixture`, `NewManager`, `atoi64`, `openTestStore`, `.handleGroupDetail`, `Transaction`, `Store`, `RandomToken`, `countRows`, `Renderer`, `gosplit/main.go`, `.netGroup`, `net/http.Request`, `Notification`, `userByEmail`, `covPair`, `rewire_test.go`, `.moveFormData`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Are the 176 inferred relationships involving `body()` (e.g. with `.stageRestore()` and `TestAdminActivityFilters()`) actually correct?**
  _`body()` has 176 INFERRED edges - model-reasoned connections that need verification._
- **Are the 120 inferred relationships involving `newHarness()` (e.g. with `TestAdminActivityRequiresAdmin()` and `TestServiceWorkerSkipsPrivateResponses()`) actually correct?**
  _`newHarness()` has 120 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `filterOpt` to the rest of the system?**
  _196 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `ctxTimeout` be split into smaller, more focused modules?**
  _Cohesion score 0.12878787878787878 - nodes in this community are weakly interconnected._