# Graph Report - goSplit  (2026-10-20)

## Corpus Check
- 234 files · ~242,278 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .example 1, .css 1)

## Summary
- 2540 nodes · 11435 edges · 131 communities (86 shown, 45 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1412 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b96791e9`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.ResponseWriter
- newTestService
- idiomorph-ext.min.js
- NewManager
- Base Layout
- Load
- atoi64
- .Dump
- testing.T
- provider_test.go
- ExpenseRecurrence
- configuration.md
- Group
- money.go
- Group Detail Page
- crypto_test.go
- time.Duration
- DecodeColumn
- database/sql query layer (as-built)
- dev.sh
- render_fragment_test.go
- Expense
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
- Service
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
- nullInt
- Renderer
- harness
- gosplit/main.go
- runBackupCLI
- bank.js
- GoSplit App Icon
- newTestRunner
- .netGroup
- Development
- Validated
- httpapp/split_inputs_test.go
- sw.js
- Architecture
- nameCache
- web/theme_test.go
- recovery_test.go
- BackupSchedule
- Client behaviour
- Notification
- countOf
- userByEmail
- runPathsCLI
- helpers.go
- covPair
- settledChainHarness
- covUserMap
- htmx.min.js
- github.com/hafio/gosplit
- JobTracker
- crypto.go
- Configuration
- Deployment
- filesOnly
- Backup and restore
- rewire_test.go
- extractToken
- NoneProvider
- net/http.Request
- main
- countingRates
- PlaidProvider
- covErrRates
- fakeRates
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
- Config
- web/zz_coverage_test.go
- pathsCfg
- body
- push/push_test.go
- ownerOf

## God Nodes (most connected - your core abstractions)
1. `body()` - 184 edges
2. `newHarness()` - 132 edges
3. `newTestService()` - 98 edges
4. `ctxTimeout()` - 70 edges
5. `Expense` - 62 edges
6. `User` - 60 edges
7. `covUser()` - 59 edges
8. `openTestStore()` - 56 edges
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

## Communities (131 total, 45 thin omitted)

### Community 0 - "net/http.ResponseWriter"
Cohesion: 0.08
Nodes (9): Server, Server, safeNext(), TestSafeNextRefusesOffSiteTargets(), Server, Server, sortedNets(), Server (+1 more)

### Community 1 - "newTestService"
Cohesion: 0.06
Nodes (59): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestArchiveNothing(), TestCreateConversionExact(), TestCreateConversionExactDatesABlankDateToday(), TestDeleteExpenseRejectsStaleVersion(), TestAddFriendToGroup(), TestToggleHiddenFriendKeepsTheOthers() (+51 more)

### Community 2 - "idiomorph-ext.min.js"
Cohesion: 0.28
Nodes (15): a(), c(), d(), e(), f(), h(), i(), l() (+7 more)

### Community 3 - "NewManager"
Cohesion: 0.09
Nodes (33): CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler(), covOpenStore(), TestCovAuthenticate_ExistingCSRFReused() (+25 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.09
Nodes (28): backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults(), TestBackupDirDifferentFromAutoRestoreDirAccepted(), TestBackupDirEqualAutoRestoreDirFailsLoud(), TestBackupRetentionValidation() (+20 more)

### Community 6 - "atoi64"
Cohesion: 0.09
Nodes (22): candidate, expenseDetailView, fieldError, participantView, applySubmittedFields(), displayName(), fieldErrorf(), fieldErrorsFor() (+14 more)

### Community 7 - ".Dump"
Cohesion: 0.15
Nodes (10): pending, queryer, WriteHeader(), NewSealWriter(), Runner, readMigrationVersions(), writeTarFile(), SweepStalePartials() (+2 more)

### Community 8 - "testing.T"
Cohesion: 0.04
Nodes (101): inspectDir(), probeDir(), TestInspectDirEmptyPath(), TestInspectDirMissing(), TestInspectDirMissingUnderAnUnwritableParent(), TestInspectDirNotADirectory(), TestInspectDirUnwritable(), TestInspectDirWritable() (+93 more)

### Community 9 - "provider_test.go"
Cohesion: 0.29
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.15
Nodes (9): Service, Service, TestGenerateDueRecurrencesSkipsABrokenTemplate(), toISO(), todayISO(), TestGetRateWithoutADateRefreshesDaily(), ExpenseRecurrence, Store (+1 more)

### Community 11 - "configuration.md"
Cohesion: 0.41
Nodes (6): app service, splitpro-data volume, All pages, GoSplit documentation, Start here, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.12
Nodes (13): currencyNet, groupListRow, groupRow, hasGroup(), inPlaceholders(), prefixCols(), trimSpace(), Store (+5 more)

### Community 13 - "money.go"
Cohesion: 0.19
Nodes (13): absInt64(), Server, allDigits(), decimals(), Format(), FormatWithCode(), Parse(), pow10() (+5 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "crypto_test.go"
Cohesion: 0.21
Nodes (23): NewOpenReader(), frameAt(), open(), seal(), TestDuplicatedFrameFails(), TestEmptyPayloadStillTerminates(), TestFingerprintDistinguishesSecrets(), TestFingerprintDoesNotRevealSealKey() (+15 more)

### Community 16 - "time.Duration"
Cohesion: 0.14
Nodes (7): FromNow(), NewNanoID(), Store, Store, TestCovIDGenerators(), Scheduler, Session

### Community 17 - "DecodeColumn"
Cohesion: 0.11
Nodes (22): Column, ColumnKind, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), ScanDest(), nullBool() (+14 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "render_fragment_test.go"
Cohesion: 0.09
Nodes (30): TestNotificationKindsHaveLocaleEntries(), Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load(), TestDetect() (+22 more)

### Community 21 - "Expense"
Cohesion: 0.10
Nodes (20): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys(), BuildLikePattern(), ExpenseFilter, HistoricalBatch (+12 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.24
Nodes (19): covRoundTripFunc, covClient(), covJSONResp(), TestFrankfurterRateMalformedJSON(), TestFrankfurterRateMissingRate(), TestFrankfurterRateStatusError(), TestFrankfurterRateSuccess(), TestFrankfurterRateTransportError() (+11 more)

### Community 23 - "schema_test.go"
Cohesion: 0.18
Nodes (16): SameMigrationSet(), InsertOrder(), isBareIdent(), TableCount(), TestDeleteOrderIsExactReverse(), TestInsertOrderSatisfiesForeignKeys(), TestInsertOrderStartsWithUsers(), TestIsBareIdent() (+8 more)

### Community 24 - "index.go"
Cohesion: 0.13
Nodes (23): countingReader, Entry, EntryKind, Index, Limits, Opener, opener, decodeManifest() (+15 more)

### Community 25 - "context.Context"
Cohesion: 0.07
Nodes (14): Service, Service, Service, Store, Store, User, Store, Store (+6 more)

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 27 - ".handleGroupDetail"
Cohesion: 0.32
Nodes (7): expenseRow, monthGroup, TestGroupByMonth(), applyFeedLimit(), groupByMonth(), parseFilter(), showAllHref()

### Community 32 - "split.go"
Cohesion: 0.07
Nodes (48): formatValue(), lineForMethod(), orZero(), prefillStored(), rebalanceToTarget(), sumTarget(), valueForMethod(), TestPrefillStoredKeepsCompleteSet() (+40 more)

### Community 35 - "mail_test.go"
Cohesion: 0.12
Nodes (11): buildMessage(), serveFakeSMTP(), startTLSRelay(), TestBuildMessage(), TestLogMailerSend(), TestNewLogMailerWhenSMTPUnset(), TestNewSMTPMailerWhenHostSet(), TestSMTPMailerDialError() (+3 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.10
Nodes (20): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+12 more)

### Community 39 - "Service"
Cohesion: 0.13
Nodes (14): ExpenseInput, Service, SettlementInput, lineIDs(), logAdminOverride(), Movable(), movable(), nullInt() (+6 more)

### Community 40 - "Transaction"
Cohesion: 0.10
Nodes (6): Disabled, Provider, Transaction, Service, bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

### Community 42 - "index_test.go"
Cohesion: 0.25
Nodes (29): craftEntry, ReadHeader(), tablePartName(), b64(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive() (+21 more)

### Community 43 - "tripHarness"
Cohesion: 0.08
Nodes (46): adminOutsiderHarness(), expenseByName(), TestAdminActivityFilters(), TestAdminActivityListsEveryoneNeutrally(), TestAdminActivityShowAllKeepsFilters(), TestAdminDeleteReturnsToAdminFeed(), TestAdminEditsAnyonesExpenseUntouched(), TestAdminEditsDirectExpenseUntouched() (+38 more)

### Community 44 - "Store"
Cohesion: 0.18
Nodes (7): friendRow, friendSettlePrefill(), CumulatedBalance, Store, scanBalances(), Store, Balance

### Community 45 - "adminHarness"
Cohesion: 0.10
Nodes (24): adminHarness(), harness, tableCounts(), TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted(), TestBackupDownloadRejectsHostileNames(), TestBackupDownloadServesArchive(), TestBackupGenerateAndDownloadRoundTrip() (+16 more)

### Community 46 - "RandomToken"
Cohesion: 0.14
Nodes (11): HashPassword(), RandomToken(), TestHashPassword_Empty(), TestHashVerifyRoundTrip(), TestRandomTokenUnique(), TestVerifyPassword_InvalidHash(), VerifyPassword(), Service (+3 more)

### Community 48 - "covfilter/main.go"
Cohesion: 0.13
Nodes (26): block, marker, pos, span, unit, applyMarkers(), directive(), main() (+18 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.57
Nodes (6): b64ToUint8(), csrf(), enable(), test(), wire(), wireOnce()

### Community 53 - "nullInt"
Cohesion: 0.09
Nodes (27): nullInt(), newOverrideFixture(), TestAdminEditsAndDeletesAnyTransaction(), TestAdminGetsNoGroupLevelPowers(), TestAdminMoveSkipsOnlyTheActorCheck(), TestAdminSettlementEditSkipsOnlyTheActorCheck(), TestIsAdminFailsClosed(), countRows() (+19 more)

### Community 54 - "Renderer"
Cohesion: 0.29
Nodes (4): Renderer, ViewData, htmlHeaders(), setServerTiming()

### Community 55 - "harness"
Cohesion: 0.14
Nodes (8): TestBackupRoutesRejectAnonymous(), harness, TestCSRFRejected(), TestFullFlow(), TestHealth(), TestLoginRequiredRedirect(), TestPageNoStoreHeaders(), TestStaticCacheHeaders()

### Community 56 - "gosplit/main.go"
Cohesion: 0.19
Nodes (11): ctxKey, balanceRow, bankTxRow, collapseView, filterOpt, recurrenceRow, templateOption, friendExport (+3 more)

### Community 57 - "runBackupCLI"
Cohesion: 0.15
Nodes (27): cliContext(), loadCLIConfig(), orDash(), printManifest(), runBackupCLI(), runBackupCommand(), runInspectCLI(), runRestoreCLI() (+19 more)

### Community 58 - "bank.js"
Cohesion: 0.83
Nodes (3): connect(), csrf(), wire()

### Community 59 - "GoSplit App Icon"
Cohesion: 0.67
Nodes (3): GoSplit App Icon, GoSplit Brand Identity, Split and Sharing Visual Metaphor

### Community 60 - "newTestRunner"
Cohesion: 0.15
Nodes (38): addCarol(), autoRestoreFixture(), mustGlob(), safetyDumps(), TestCheckAutoRestoreFailuresAbortTheBoot(), TestCheckAutoRestoreLeavesItToTheLockHolder(), TestCheckAutoRestoreRestoresOnceAndRecordsIt(), TestCheckAutoRestoreWithoutAnArchiveDoesNothing() (+30 more)

### Community 61 - ".netGroup"
Cohesion: 0.24
Nodes (4): nullStr(), Service, Service, nullableString()

### Community 62 - "Development"
Cohesion: 0.13
Nodes (15): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Dev scripts, Development (+7 more)

### Community 63 - "Validated"
Cohesion: 0.16
Nodes (9): ApplyOptions, batcher, Report, Table, Validated, orDashStr(), writeMarker(), Runner (+1 more)

### Community 64 - "httpapp/split_inputs_test.go"
Cohesion: 0.15
Nodes (23): seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord(), deletedCount() (+15 more)

### Community 66 - "Architecture"
Cohesion: 0.11
Nodes (19): Architecture, Background jobs, Backup archives, Balances are derived, never stored, Collapsing history, Data model, Decisions worth keeping, Internationalisation (+11 more)

### Community 67 - "nameCache"
Cohesion: 0.10
Nodes (12): nameCache, notificationRow, TestFeedDateParts(), Server, dateOnly(), feedDateParts(), Server, monthLabel() (+4 more)

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
Cohesion: 0.12
Nodes (9): Service, notificationsFor(), TestNotificationsForSkipsActorAndEmpty(), Store, nowISO(), Notification, Store, scanNotification() (+1 more)

### Community 73 - "countOf"
Cohesion: 0.31
Nodes (9): TestAdminCannotAddToForeignGroup(), TestAdminViewsAnyGroupReadOnly(), countOf(), groupFlags(), outsiderHarness(), TestGroupActionsRejectOutsider(), TestGroupMembersHiddenFromOutsider(), TestRestoreRoutesRefuseBackupTokens() (+1 more)

### Community 74 - "userByEmail"
Cohesion: 0.21
Nodes (15): limitedHarness(), TestAdminConsoleLimitedRole(), TestLimitedCannotCreateGroup(), TestLimitedPagesHideGroupCreationAndImport(), reread(), seedNotification(), TestBellBadgeAppearsOnEveryPage(), TestNotificationBadgeAndMenuFragments() (+7 more)

### Community 75 - "runPathsCLI"
Cohesion: 0.12
Nodes (17): humanBytes(), TestHumanBytes(), printDirContents(), printDirStatus(), runPathsCLI(), capture(), captureStdout(), TestPrintDirContentsEmpty() (+9 more)

### Community 76 - "helpers.go"
Cohesion: 0.17
Nodes (12): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+4 more)

### Community 77 - "covPair"
Cohesion: 0.29
Nodes (11): covInput(), covNotifications(), covPair(), pushSink(), TestNotifyAddExpenseRecordsForOthersOnly(), TestNotifyDeletedExpenseKeepsItsNotification(), TestNotifyEmailIsOptIn(), TestNotifyEventKinds() (+3 more)

### Community 78 - "settledChainHarness"
Cohesion: 0.60
Nodes (4): nettingBetween(), settledChainHarness(), TestNettingEntryRefusesChanges(), TestNettingRowsRenderReadOnly()

### Community 79 - "covUserMap"
Cohesion: 0.31
Nodes (10): covNotif(), covOptInEmail(), covUserMap(), TestNotifyNothingDeliveredWithoutARow(), TestPushPrunesGoneSubscriptions(), TestShareLineBranches(), TestCovEmailNotifications(), TestCovPushGating() (+2 more)

### Community 80 - "htmx.min.js"
Cohesion: 0.07
Nodes (106): Cutting a release, GitHub Actions, Versioning and release, a(), Ae(), an(), at(), B() (+98 more)

### Community 82 - "JobTracker"
Cohesion: 0.15
Nodes (8): JobState, TableStat, Manifest, JobStatus, JobTracker, newToken(), removeQuietly(), TestTokensAreUnguessable()

### Community 83 - "crypto.go"
Cohesion: 0.10
Nodes (12): Keyring, openReader, sealWriter, expand(), newAEAD(), NewKeyring(), NewNoncePrefix(), NewSalt() (+4 more)

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
Cohesion: 0.09
Nodes (47): pairKey, Row, Shift, Transfer, settlementRow, Pair, lessKey(), NeedsTidy() (+39 more)

### Community 89 - "extractToken"
Cohesion: 0.24
Nodes (8): extractToken(), TestAdminAutoPromotionNeedsAVerifiedAddress(), TestForgotResetPassword(), TestMagicLinkFlow(), TestCovAuthErrorBranches(), TestCovAuthInactiveAndPromotion(), TestAdminMagicLinkForUser(), fakeMailer

### Community 91 - "net/http.Request"
Cohesion: 0.13
Nodes (8): archiveInfo, Server, writeJobJSON(), Server, fragmentTarget(), Server, isBackgroundRequest(), navSlug()

### Community 92 - "main"
Cohesion: 0.29
Nodes (7): TestWantsBackup(), TestWantsBackupAndVersionAreDisjoint(), wantsBackup(), main(), TestVersionDefault(), TestWantsVersion(), wantsVersion()

### Community 94 - "PlaidProvider"
Cohesion: 0.17
Nodes (4): PlaidProvider, FrankfurterProvider, OpenExchangeRatesProvider, swGet()

### Community 99 - "New"
Cohesion: 0.10
Nodes (30): parseLogLevel(), run(), defaultClient(), Provider, NewProvider(), TestNewProviderSelection(), New(), Mailer (+22 more)

### Community 128 - "PruneOldArchives"
Cohesion: 0.39
Nodes (9): PruneOldArchives(), lsNames(), TestPruneIgnoresOtherPrefixes(), TestPruneIgnoresUnrelatedFiles(), TestPruneKeepsEverythingWhenDisabled(), TestPruneKeepsNewest(), TestPruneMissingDirIsNotAnError(), TestPruneNoopWhenUnderRetention() (+1 more)

### Community 129 - "Config"
Cohesion: 0.11
Nodes (23): warnPendingMigrations(), acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), TestFindAutoRestoreArchive(), New(), newTestRunnerIn(), Config (+15 more)

### Community 132 - "web/zz_coverage_test.go"
Cohesion: 0.14
Nodes (17): currencyCodes(), methodGlyph(), Asset(), csvRows(), covRenderer(), TestCovAsset(), TestCovAssetsHandlerCacheControl(), TestCovCSVRows() (+9 more)

### Community 134 - "pathsCfg"
Cohesion: 0.26
Nodes (13): prepareBoot(), prepareDataDirs(), collectDirStatuses(), pathsCfg(), TestCollectDirStatusesCoversEverySetting(), TestCollectDirStatusesOmitsDatabaseDirForPostgres(), TestCollectDirStatusesReportsUnsetAutoRestore(), TestPrepareBootCreatesTheDirectories() (+5 more)

### Community 137 - "body"
Cohesion: 0.04
Nodes (101): TestAdminActivityRequiresAdmin(), TestServiceWorkerSkipsPrivateResponses(), TestMemberAdminKeepsGroupControls(), TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse() (+93 more)

### Community 141 - "push/push_test.go"
Cohesion: 0.18
Nodes (12): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDeliversAndReportsGone(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError() (+4 more)

## Knowledge Gaps
- **196 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `filterOpt`, `collapseView`, `bankTxRow` (+191 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 299 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **45 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ctxTimeout()` connect `net/http.ResponseWriter` to `nameCache`, `atoi64`, `.handleGroupDetail`, `money.go`, `gosplit/main.go`, `context.Context`, `net/http.Request`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Why does `covMultipartReq()` connect `testing.T` to `gosplit/main.go`, `net/http.Request`?**
  _High betweenness centrality (0.024) - this node is a cross-community bridge._
- **Why does `Expense` connect `Expense` to `split.go`, `newTestService`, `nameCache`, `atoi64`, `Service`, `Notification`, `testing.T`, `rewire_test.go`, `tripHarness`, `settledChainHarness`, `nullInt`, `gosplit/main.go`, `.handleGroupDetail`, `.netGroup`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._
- **Are the 174 inferred relationships involving `body()` (e.g. with `.stageRestore()` and `TestAdminActivityFilters()`) actually correct?**
  _`body()` has 174 INFERRED edges - model-reasoned connections that need verification._
- **Are the 119 inferred relationships involving `newHarness()` (e.g. with `TestAdminActivityRequiresAdmin()` and `TestServiceWorkerSkipsPrivateResponses()`) actually correct?**
  _`newHarness()` has 119 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `filterOpt` to the rest of the system?**
  _196 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.07936507936507936 - nodes in this community are weakly interconnected._