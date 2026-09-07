# Graph Report - gosplit  (2026-09-07)

## Corpus Check
- 203 files · ~193,480 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2163 nodes · 6076 edges · 147 communities (91 shown, 42 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 1122 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `55553fc9`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.Request
- newTestService
- handlers_expenses.go
- NewManager
- Base Layout
- Load
- Server
- notify_test.go
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
- httpapp/notifications_test.go
- Development
- covUser
- seedDirectExpense
- sw.js
- Architecture
- Notification
- htmx.min.js
- github.com/hafio/gosplit
- JobTracker
- Keyring
- Configuration
- Deployment
- TestThemeColorUpdate
- Backup and restore
- .handleGroupDetail
- Config
- Server
- Manifest
- .Dump
- PlaidProvider
- .buildBatches
- helpers.go
- New
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
- prune_test.go
- Store
- EnsureWritableDir
- TestCov3BankConvertRedirect
- Renderer
- GoSplit documentation
- run
- Server
- web/zz_coverage_test.go
- body
- view.go
- NewRenderer
- ValidTheme
- New
- ownerOf
- .buildNotificationRows
- Four ways to make a backup
- TestDeleteExpenseAuthorization
- .CreateConversionExact

## God Nodes (most connected - your core abstractions)
1. `body()` - 141 edges
2. `newHarness()` - 117 edges
3. `newTestService()` - 84 edges
4. `ctxTimeout()` - 68 edges
5. `User` - 50 edges
6. `covUser()` - 49 edges
7. `Expense` - 49 edges
8. `openTestStore()` - 48 edges
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

## Communities (147 total, 42 thin omitted)

### Community 0 - "net/http.Request"
Cohesion: 0.09
Nodes (15): context.CancelFunc, net/http.Request, net/http.ResponseWriter, Server, Server, safeNext(), Server, absInt64() (+7 more)

### Community 1 - "newTestService"
Cohesion: 0.10
Nodes (28): TestAdminCreateUser(), TestAdminSetPasswordRevokesSessions(), TestCreateConversionExact(), TestAddFriendToGroup(), TestEditSameGroupNoAck(), TestMoveExpenseEditsInPlace(), TestMoveExpenseGroupMemberNotParticipant(), TestMoveExpenseNotMovableWhenDeleted() (+20 more)

### Community 2 - "handlers_expenses.go"
Cohesion: 0.13
Nodes (24): fieldError, applySubmittedFields(), fieldErrorf(), formatValue(), formVersion(), lineForMethod(), orZero(), parseExpenseInput() (+16 more)

### Community 3 - "NewManager"
Cohesion: 0.12
Nodes (31): ctxKey, net/http.Handler, CSRFFrom(), Manager, NewManager(), UserFrom(), covNewUser(), covOKHandler() (+23 more)

### Community 4 - "Base Layout"
Cohesion: 0.08
Nodes (40): Balance, Group, Groups Page, Route GET /groups/{id} (Group Detail), Route POST /groups/create, Splitwise Import Page, Route POST /import/splitwise, Base Layout (+32 more)

### Community 5 - "Load"
Cohesion: 0.11
Nodes (33): backupEnv(), TestBackupCronAccepted(), TestBackupCronInvalidFailsLoud(), TestBackupCronWithoutDirFailsLoud(), TestBackupDefaults(), TestBackupDirDifferentFromAutoRestoreDirAccepted(), TestBackupDirEqualAutoRestoreDirFailsLoud(), TestBackupRetentionValidation() (+25 more)

### Community 6 - "Server"
Cohesion: 0.18
Nodes (5): candidate, displayName(), fieldErrorsFor(), Server, prefillFromForm()

### Community 7 - "notify_test.go"
Cohesion: 0.18
Nodes (23): covInput(), covNotif(), covNotifications(), covOptInEmail(), covPair(), covUserMap(), Service, TestNotificationsForSkipsActorAndEmpty() (+15 more)

### Community 8 - "openTestStore"
Cohesion: 0.05
Nodes (68): Transfer, Simplify(), netAfter(), TestSimplify_GoldenUSDTransactionCount(), TestSimplify_PreservesNetAndSettles(), TestUserNetByExpense(), TestBuildLikePattern(), TestExpenseFilterLimit() (+60 more)

### Community 9 - "provider_test.go"
Cohesion: 0.29
Nodes (12): roundTripFunc, New(), jsonResp(), plaidStub(), TestBaseURL(), TestCreateLinkToken(), TestExchangePublicToken(), TestFetchTransactions() (+4 more)

### Community 10 - "ExpenseRecurrence"
Cohesion: 0.16
Nodes (6): Service, toISO(), todayISO(), ExpenseRecurrence, Store, scanRecurrence()

### Community 11 - "configuration.md"
Cohesion: 0.60
Nodes (3): app service, splitpro-data volume, SplitPro (Go rebuild)

### Community 12 - "Group"
Cohesion: 0.17
Nodes (6): prefixCols(), trimSpace(), Store, scanGroup(), Group, TestCov2PureHelpers()

### Community 13 - "Format"
Cohesion: 0.26
Nodes (11): friendSettlePrefill(), decimals(), Format(), FormatWithCode(), Parse(), pow10(), TestFormat(), TestFormatWithCode() (+3 more)

### Community 14 - "Group Detail Page"
Cohesion: 0.08
Nodes (44): Activity Item Entity, Activity Page, Admin Users Page, Balance Entity, Balances Page, Bank Link Client Script, Bank Transactions Page, Bank Transaction Entity (+36 more)

### Community 15 - "testKeyring"
Cohesion: 0.24
Nodes (20): frameAt(), seal(), TestDuplicatedFrameFails(), TestEmptyPayloadStillTerminates(), TestFingerprintDistinguishesSecrets(), TestFingerprintDoesNotRevealSealKey(), TestFingerprintIsStable(), TestFlippedFinalFlagFails() (+12 more)

### Community 16 - "nowISO"
Cohesion: 0.10
Nodes (12): time.Duration, FromNow(), NewNanoID(), NewUUID(), nowISO(), Store, Store, Store (+4 more)

### Community 17 - "codec_test.go"
Cohesion: 0.17
Nodes (23): database/sql.NullBool, encoding/json.RawMessage, DecodeColumn(), EncodeColumn(), isJSONNull(), isJSONSpace(), ScanDest(), nullBool() (+15 more)

### Community 19 - "dev.sh"
Cohesion: 0.16
Nodes (26): c(), docker_linux(), expand(), finish(), host_arch(), host_os(), log_begin(), NO_COLOR (+18 more)

### Community 20 - "service_test.go"
Cohesion: 0.16
Nodes (11): extractToken(), TestAdminAutoPromotion(), TestChangePassword(), TestCreateConversion(), TestForgotResetPassword(), TestImportFromSplitwise(), TestMagicLinkFlow(), TestRegisterAndLogin() (+3 more)

### Community 21 - "Expense"
Cohesion: 0.17
Nodes (11): prefillDerived(), prefillEdit(), BuildLikePattern(), ExpenseFilter, HistoricalBatch, Store, inPlaceholders(), scanExpense() (+3 more)

### Community 22 - "currency/zz_coverage_test.go"
Cohesion: 0.11
Nodes (27): covRoundTripFunc, FrankfurterProvider, NoneProvider, OpenExchangeRatesProvider, net/http.Client, defaultClient(), Provider, NewProvider() (+19 more)

### Community 23 - "newTestRunner"
Cohesion: 0.07
Nodes (62): ApplyOptions, batcher, Column, ColumnKind, Report, Table, database/sql.Tx, countRows() (+54 more)

### Community 24 - "Bundle"
Cohesion: 0.18
Nodes (11): Bundle, Load(), parseAcceptLanguage(), parseQ(), firstOther(), load(), TestDetect(), TestLanguages() (+3 more)

### Community 25 - "context.Context"
Cohesion: 0.07
Nodes (16): context.Context, Service, Service, swGet(), Service, Store, Store, Store (+8 more)

### Community 26 - "convert.js"
Cohesion: 0.37
Nodes (13): dec(), fetchRate(), fmt(), labels(), num(), recomputeTo(), setSrc(), setValue() (+5 more)

### Community 32 - "Method"
Cohesion: 0.12
Nodes (34): TestSplitRemainderStableAcrossLineOrder(), EncodeInputs(), Line, LineFromInput(), TestDecodeInputsRejects(), TestEncodeDecodeInputs(), TestEncodeInputsSkipsSystemMethods(), TestLineInputEqualAndUnknown() (+26 more)

### Community 34 - "SQLite default (no DB container)"
Cohesion: 0.67
Nodes (3): Postgres db service (optional, commented), SQLite default (no DB container), SQLite (pure-Go modernc.org/sqlite)

### Community 35 - "mail_test.go"
Cohesion: 0.13
Nodes (15): crypto/tls.Config, net.Listener, net/smtp.Client, buildMessage(), Mailer, New(), serveFakeSMTP(), TestBuildMessage() (+7 more)

### Community 37 - "dev.ps1"
Cohesion: 0.18
Nodes (18): Get-Log(), Get-Now(), Invoke-Logged(), Build-Image(), Ok(), Task-build(), Task-cov(), Task-down() (+10 more)

### Community 38 - "Features"
Cohesion: 0.11
Nodes (19): Admin console, Balances and activity, Bank sync (optional), Collapsing history, Currency conversion, Expenses, Features, Filtering (+11 more)

### Community 39 - "Service"
Cohesion: 0.20
Nodes (11): database/sql.NullInt64, database/sql.NullString, ExpenseInput, Service, SettlementInput, movable(), nullInt(), nullStr() (+3 more)

### Community 40 - "Transaction"
Cohesion: 0.18
Nodes (5): Transaction, Service, itoa(), bankData, covBank

### Community 41 - "expense_form.js"
Cohesion: 0.44
Nodes (9): amountMinor(), currentMethod(), decimals(), fmt(), renderPreview(), symbol(), wire(), wireOnce() (+1 more)

### Community 42 - "index_test.go"
Cohesion: 0.33
Nodes (22): craftEntry, tablePartName(), assertNothingOutside(), baselineEntries(), baselineManifest(), craftArchive(), itoa(), scanCrafted() (+14 more)

### Community 43 - "tripHarness"
Cohesion: 0.13
Nodes (31): harness, settlementPath(), TestEditSettlementGroupMoveNeedsAck(), TestEditSettlementHidesFixedFieldPickers(), TestEditSettlementKeepsZeroDecimalCurrency(), TestEditSettlementNonMemberRendersFieldError(), TestEditSettlementPageHasNoMethodPicker(), TestEditSettlementRejectsBadAmount() (+23 more)

### Community 44 - "Store"
Cohesion: 0.24
Nodes (5): friendRow, CumulatedBalance, Store, scanBalances(), Balance

### Community 45 - "testing.T"
Cohesion: 0.13
Nodes (28): testing.T, TestConvertExactBothAmounts(), TestRateEndpoint(), adminHarness(), harness, harness, TestAdminPageLinksToBackup(), TestBackupDownloadLinkNotBoosted() (+20 more)

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
Cohesion: 0.16
Nodes (24): countingReader, Entry, EntryKind, Index, Limits, Opener, Validated, archive/tar.Reader (+16 more)

### Community 50 - "GoSplit UI Icon Sprite Sheet"
Cohesion: 0.40
Nodes (5): Expense & Money Icons (banknote, wallet), Group & Member Icons (users, user-plus), GoSplit UI Icon Sprite Sheet, Recurring Expense Icon (repeat), Settings / Filter Icon (sliders)

### Community 51 - "push.js"
Cohesion: 0.57
Nodes (6): b64ToUint8(), csrf(), enable(), test(), wire(), wireOnce()

### Community 53 - "paths_cli_test.go"
Cohesion: 0.15
Nodes (25): prepareDataDirs(), collectDirStatuses(), inspectDir(), printDirContents(), printDirStatus(), probeDir(), runPathsCLI(), captureStdout() (+17 more)

### Community 54 - "service/zz_coverage2_test.go"
Cohesion: 0.22
Nodes (7): covMultipartReq(), TestCovBankConnectedFlow(), TestCovBankProviderErrors(), TestCovConversionErrors(), TestCovInviteSendsPendingInvites(), TestCovUpdateAvatar(), covErrRates

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

### Community 61 - "httpapp/notifications_test.go"
Cohesion: 0.25
Nodes (17): harness, reread(), seedNotification(), TestBellBadgeAppearsOnEveryPage(), TestNotificationBadgeAndMenuFragments(), TestNotificationEntityHrefFallback(), TestNotificationKindsHaveLocaleEntries(), TestNotificationOpenIsScopedToItsOwner() (+9 more)

### Community 62 - "Development"
Cohesion: 0.11
Nodes (18): Adding a configuration setting, Adding a fragment region, Adding a locale, Adding a page, Adding a table, Conventions, Cutting a release, Dev scripts (+10 more)

### Community 63 - "covUser"
Cohesion: 0.20
Nodes (17): covUser(), Service, TestAddExpenseDirectCreatesFriendship(), TestAddExpenseNonMemberRejected(), TestAddFriendByEmail(), TestAdminMagicLinkForUser(), TestArchiveDirectRejectsSelf(), TestArchiveGroupNonMember() (+9 more)

### Community 64 - "seedDirectExpense"
Cohesion: 0.15
Nodes (24): harness, seedDirectExpense(), TestEditCrossGroupRequiresAck(), TestEditPagePrefillNoAck(), TestEditPageRetargetResetsToEqual(), TestEditPercentagePrefillSumsTo100(), TestEditSameGroupNoAckHTTP(), TestMoveKeepsRecord() (+16 more)

### Community 66 - "Architecture"
Cohesion: 0.12
Nodes (17): Architecture, Background jobs, Backup archives, Balances are derived, never stored, Collapsing history, Data model, Decisions worth keeping, Internationalisation (+9 more)

### Community 72 - "Notification"
Cohesion: 0.27
Nodes (4): Service, notificationsFor(), Notification, scanNotification()

### Community 80 - "htmx.min.js"
Cohesion: 0.06
Nodes (118): a(), Ae(), an(), at(), B(), Be(), bn(), bt() (+110 more)

### Community 82 - "JobTracker"
Cohesion: 0.13
Nodes (20): JobState, pending, sync.Mutex, JobStatus, JobTracker, NewJobTracker(), newToken(), removeQuietly() (+12 more)

### Community 83 - "Keyring"
Cohesion: 0.13
Nodes (19): Keyring, openReader, sealWriter, crypto/cipher.AEAD, io.WriteCloser, io.Writer, expand(), newAEAD() (+11 more)

### Community 84 - "Configuration"
Cohesion: 0.13
Nodes (15): Accounts and features, Backup and restore, Bank sync (Plaid, optional), Configuration, Core, Currency rates, Database, Email (SMTP) (+7 more)

### Community 85 - "Deployment"
Cohesion: 0.13
Nodes (15): Bind mounts must be owned by 65532, Choosing an engine, Deployment, Docker Compose, First admin, Health check, Mail, Monitoring and logs (+7 more)

### Community 87 - "Backup and restore"
Cohesion: 0.14
Nodes (14): Archives are secrets, Backup and restore, Check first, Housekeeping, Inspecting an archive, Migration sets must match exactly, Not supported, by decision, Replace everything (+6 more)

### Community 88 - ".handleGroupDetail"
Cohesion: 0.10
Nodes (24): balanceRow, currencyNet, expenseDetailView, expenseRow, groupRow, monthGroup, nameCache, participantView (+16 more)

### Community 89 - "Config"
Cohesion: 0.16
Nodes (20): Config, Engine, TestAppliedMigrationsMatchesEmbedded(), TestEmbeddedMigrationsAreSortedAndNamed(), TestOpenMigratesAndOpenNoMigrateDoesNot(), TestOpenNoMigrateLeavesSchemaAlone(), TestPendingMigrationsOnEmptyDatabase(), TestRebindMatchesUnexported() (+12 more)

### Community 91 - "Server"
Cohesion: 0.20
Nodes (3): archiveInfo, Server, writeJobJSON()

### Community 92 - "Manifest"
Cohesion: 0.25
Nodes (7): Header, TableStat, decodeB64(), Manifest, ReadHeader(), WriteHeader(), TestReadHeaderRejectsBadContainers()

### Community 93 - ".Dump"
Cohesion: 0.24
Nodes (10): queryer, archive/tar.Writer, time.Time, b64(), Runner, readMigrationVersions(), SameMigrationSet(), writeTarFile() (+2 more)

### Community 94 - "PlaidProvider"
Cohesion: 0.17
Nodes (3): Disabled, PlaidProvider, Provider

### Community 95 - ".buildBatches"
Cohesion: 0.29
Nodes (5): buildHistoricalCSV(), dateOnly(), Service, sortedInt64Keys(), sortedStrKeys()

### Community 96 - "helpers.go"
Cohesion: 0.19
Nodes (12): avatarColor(), categories(), categoryEmoji(), firstAlnum(), initials(), TestAssetURLFingerprinted(), TestAvatarColorStableAndInRange(), TestCategoriesNonEmpty() (+4 more)

### Community 97 - "New"
Cohesion: 0.22
Nodes (7): New(), TestCovAuthErrorBranches(), TestCovDispatchDefaults(), TestCovImportBranches(), TestCovImportSwGetErrors(), TestCovRecurrenceErrorBranches(), covFailMailer

### Community 98 - "CheckAutoRestore"
Cohesion: 0.60
Nodes (5): acquireWithRenewal(), CheckAutoRestore(), findAutoRestoreArchive(), orDashStr(), writeMarker()

### Community 99 - "New"
Cohesion: 0.17
Nodes (23): sync/atomic.Bool, archiveCount(), newBackupSchedService(), TestMaybeBackupNotYetDue(), TestNewWithCronSchedulesNextRun(), TestNewWithInvalidCronDisablesBackupsWithoutPanicking(), TestNewWithoutCronDisablesBackups(), TestScheduledBackupDoesNotBlockTick() (+15 more)

### Community 127 - "Client behaviour"
Cohesion: 0.29
Nodes (7): Client behaviour, Fragments, Freshness, Measurement, Navigation, Progressive web app, Safeguards

### Community 128 - "prune_test.go"
Cohesion: 0.27
Nodes (12): PruneOldArchives(), SweepStalePartials(), lsNames(), TestPruneIgnoresOtherPrefixes(), TestPruneIgnoresUnrelatedFiles(), TestPruneKeepsEverythingWhenDisabled(), TestPruneKeepsNewest(), TestPruneMissingDirIsNotAnError() (+4 more)

### Community 129 - "Store"
Cohesion: 0.22
Nodes (4): database/sql.DB, New(), Service, Store

### Community 130 - "EnsureWritableDir"
Cohesion: 0.27
Nodes (12): EnsureWritableDir(), probeWritable(), mustBePOSIXPermissions(), TestDumpToDirFailsFastOnUnwritableDir(), TestEnsureWritableDirAcceptsWritable(), TestEnsureWritableDirCreatesMissing(), TestEnsureWritableDirNamesEachSetting(), TestEnsureWritableDirRejectsEmpty() (+4 more)

### Community 131 - "TestCov3BankConvertRedirect"
Cohesion: 0.60
Nodes (4): covSeedBankTx(), harness, TestCov3BankConvertRedirect(), TestCov3BankPageWithCachedTx()

### Community 132 - "Renderer"
Cohesion: 0.29
Nodes (5): html/template.Template, Renderer, ViewData, htmlHeaders(), setServerTiming()

### Community 133 - "GoSplit documentation"
Cohesion: 0.67
Nodes (3): All pages, GoSplit documentation, Start here

### Community 134 - "run"
Cohesion: 0.23
Nodes (10): TestWantsBackup(), TestWantsBackupAndVersionAreDisjoint(), wantsBackup(), main(), parseLogLevel(), run(), TestVersionDefault(), TestWantsVersion() (+2 more)

### Community 135 - "Server"
Cohesion: 0.16
Nodes (7): net/http.HandlerFunc, maxUploadBytes(), cacheControl(), fragmentTarget(), Server, isBackgroundRequest(), navSlug()

### Community 136 - "web/zz_coverage_test.go"
Cohesion: 0.21
Nodes (13): currencyCodes(), methodGlyph(), covRenderer(), TestCovAssetsHandlerCacheControl(), TestCovCurrencyCodes(), TestCovFuncsAbs64(), TestCovFuncsDict(), TestCovMethodGlyph() (+5 more)

### Community 137 - "body"
Cohesion: 0.06
Nodes (77): TestAdminCreateUser(), TestAdminRequiresAdmin(), TestAdminSetPasswordRevokesTargetSession(), TestAdminUserManagement(), TestExpenseNoteAndCollapse(), TestArchivedGroupHiddenFromActivityAndSection(), TestExpenseFormRendersComponents(), TestExpenseFormRoundTripsMethods() (+69 more)

### Community 138 - "view.go"
Cohesion: 0.16
Nodes (12): TestQueryWithout(), TestFragmentForLookup(), TestPollableCoversListPagesOnly(), Asset(), csvRows(), FragmentFor(), pollable(), queryWithout() (+4 more)

### Community 139 - "NewRenderer"
Cohesion: 0.42
Nodes (10): feedViewData(), TestFragmentMatchesRegionInFullPage(), TestFragmentRegistryIsValid(), TestFragmentVersionTracksContent(), TestRenderFragmentOmitsLayout(), TestRenderFragmentReportsTemplateFailure(), TestRenderFragmentSetsNoStoreHeaders(), TestRenderFragmentUnknownFailsLoudly() (+2 more)

### Community 140 - "ValidTheme"
Cohesion: 0.29
Nodes (8): TestDefaultThemeIsValid(), TestThemeHex(), TestThemesComplete(), TestValidTheme(), themeHex(), Themes(), ValidTheme(), Theme

### Community 141 - "New"
Cohesion: 0.22
Nodes (9): Sender, New(), TestNewDisabled(), TestNewEnabledExplicitEmail(), TestNewEnabledFromEmailFallback(), TestSendDisabledIsNoop(), TestSendEnabledDeliveryError(), TestSendInvalidSubscriptionJSON() (+1 more)

### Community 142 - "ownerOf"
Cohesion: 0.40
Nodes (3): ownerOf(), ownerOf(), io/fs.FileInfo

### Community 143 - ".buildNotificationRows"
Cohesion: 0.31
Nodes (3): notificationRow, entityHref(), Server

### Community 144 - "Four ways to make a backup"
Cohesion: 0.33
Nodes (6): Four ways to make a backup, From the admin page, From the command line, On a schedule, On startup (auto-restore), Seeing where things are

## Knowledge Gaps
- **191 isolated node(s):** `github.com/hafio/gosplit`, `ctxKey`, `collapseView`, `bankTxRow`, `balanceRow` (+186 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 301 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `covMultipartReq()` connect `service/zz_coverage2_test.go` to `net/http.Request`, `testing.T`?**
  _High betweenness centrality (0.040) - this node is a cross-community bridge._
- **Why does `Open()` connect `Config` to `newTestService`, `Store`, `New`, `NewManager`, `run`, `openTestStore`, `body`, `newTestRunner`, `context.Context`?**
  _High betweenness centrality (0.036) - this node is a cross-community bridge._
- **Why does `Config` connect `Config` to `Store`, `CheckAutoRestore`, `mail_test.go`, `New`, `Load`, `New`, `Server`, `provider_test.go`, `New`, `nowISO`, `Scan`, `paths_cli_test.go`, `harness`, `newTestRunner`, `New`, `.Dump`, `PlaidProvider`?**
  _High betweenness centrality (0.025) - this node is a cross-community bridge._
- **Are the 131 inferred relationships involving `body()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`body()` has 131 INFERRED edges - model-reasoned connections that need verification._
- **Are the 104 inferred relationships involving `newHarness()` (e.g. with `TestAdminCreateUser()` and `TestAdminRequiresAdmin()`) actually correct?**
  _`newHarness()` has 104 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/hafio/gosplit`, `ctxKey`, `collapseView` to the rest of the system?**
  _191 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.Request` be split into smaller, more focused modules?**
  _Cohesion score 0.0887719298245614 - nodes in this community are weakly interconnected._