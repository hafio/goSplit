# GoSplit backup and restore (+ surface the build version)

Status: implemented 2026-09-04. Executor: Opus (medium). Ledger reflects what shipped.

## Context

Two things prompted this work.

**1. Version visibility.** The build version is already captured and already derives from the
git tag -- it just never reaches the UI. [main.go:30](cmd/gosplit/main.go#L30) declares
`var version = "dev"`, stamped at link time with `-ldflags "-X main.version=$VERSION"` by
[dev.sh:90](scripts/dev.sh#L90), [dev.ps1:118](scripts/dev.ps1#L118) and
[Dockerfile:37](Dockerfile#L37). The value comes from `git describe --tags --always --dirty`
([dev.sh:76](scripts/dev.sh#L76), [Dockerfile:30](Dockerfile#L30)); the Dockerfile hard-fails
the build when it cannot be determined, and `.dockerignore` deliberately keeps `.git` in the
build context so that works. Today it only escapes via `gosplit version|-version|--version`
([main.go:33-48](cmd/gosplit/main.go#L33-L48)) and the startup log line
([main.go:79](cmd/gosplit/main.go#L79)). It is absent from `/healthz`
([server.go:283-290](internal/httpapp/server.go#L283-L290)) and the footer
([layout.html:58-60](internal/web/templates/layout.html#L58-L60)), so there is no way to tell
from a running container which build is serving.

**2. Backup and restore.** There is no way to get data out of, or back into, either engine as a
unit. The nearest thing, `GET /profile/export`
([handlers_pages.go:542-554](internal/httpapp/handlers_pages.go#L542-L554) ->
[profile_service.go:57-85](internal/service/profile_service.go#L57-L85)), is per-user, covers 4
of 21 tables, swallows errors (`friends, _ := ...`), and marshals `store` structs whose
`sql.Null*` fields carry no JSON tags -- so they serialize as `{"String":"...","Valid":true}`
and do not round-trip. There is no import counterpart. No disaster-recovery path exists if a
volume is lost, no way to move an instance between SQLite and Postgres, and no way to snapshot
before a risky admin action.

The intended outcome: one encrypted archive capturing an entire instance -- every table plus
the upload tree -- restorable completely onto either engine, from three triggers plus an
unattended startup path. Because a restore is irreversible and driven by a file that could come
from anywhere, every layer treats the archive as hostile input.

## Decided requirements

| # | Decision |
|---|---|
| R1 | Three triggers over one shared package: CLI subcommand, admin panel, scheduled |
| R2 | Scope: every table verbatim, including `sessions`, `verification_tokens`, `scheduler_locks`, `cached_currency_rates`, `cached_bank_data`, `schema_migrations`. Not `balance_view` |
| R3 | Restore: wipe and replace, ID-preserving, one transaction, full rollback, explicit confirmation |
| R4 | Uploads (`UPLOAD_DIR` tree) included under an `uploads/` prefix; restore rewrites the tree |
| R5 | Always encrypted; key derived from `SESSION_SECRET`, no override. Manifest carries a non-secret key fingerprint so a wrong-secret restore fails actionably |
| R6 | Startup auto-restore: archive present + marker absent -> restore, then create marker. Bare presence check, archive left in place |
| R7 | Cross-engine portable. Tests: SQLite round-trip + engine-agnostic value-encoding unit tests. No live Postgres harness |
| R8 | Scheduled backup configured by a 5-field cron rule plus a retention count |
| R9 | **Any** startup auto-restore failure aborts boot with an actionable error and exit 1 -- schema-version mismatch included |
| R10 | Version surfaced in the layout footer and the `/healthz` JSON body |

## Ledger

| Step | Status | Notes |
|---|---|---|
| 1. `store`: `Rebind`, `OpenNoMigrate`, `SQLiteReadOnlyDSN`, migration introspection | done | `PendingMigrations` added beyond the plan, so backup can refuse rather than migrate |
| 2. Config: backup env vars, `AppVersion`, fail-loud validation | done | also refuses `BACKUP_DIR == AUTO_RESTORE_DIR` |
| 3. `internal/backup/schema.go`: 21-table registry | done | `PKLen` added so the dump orders by the real primary key |
| 4. `internal/backup/codec.go`: value encoding | done | `KindNullInt64` added -- the plan's kind list omitted nullable integers |
| 5. `internal/backup/crypto.go` | done | chunked AEAD instead of one `Seal`; see Deviations |
| 6. `container.go` + `index.go` | done | manifest written last; table data split into bounded parts |
| 7. `dump.go`: snapshot-isolated dump | done | |
| 8. `restore.go`: validate / apply | done | |
| 9. `recovery.go`: interrupted-swap repair | done | called from server boot AND the CLI restore |
| 10. CLI subcommands | done | `backup`, `restore`, `inspect` |
| 11. Startup auto-restore | done | fatal on any failure, per R9 |
| 12. Scheduled backup + retention | done | `prune.go`, own renewed lock, runs off-tick |
| 13. Admin panel | done | async jobs, token-gated status, 39 i18n keys across 9 locales |
| 14. Maintenance-mode write gate | done | 503 + `Retry-After` on mutating requests |
| 14b. Job/upload janitor | done | added after the fact: `Prune` existed but nothing called it, so abandoned uploads leaked |
| 15. Version in footer and `/healthz` | done | |
| 16. Tests | done | 125 new test functions |
| 17. Docs | done | README, `.env.example`, `docker-compose.yml`, this file |

## Deviations from the plan as approved

- **R9 kept literally, against the hardened design's advice.** The design proposed treating a
  schema-version mismatch as "log loud and boot normally", because the marker is written only on
  success and an aborting mismatch therefore crash-loops the container forever against the same
  file -- under `restart: unless-stopped` that is an outage with no automatic exit. The decision
  is to abort on every failure, mismatch included. Accepted, with one compensating measure that
  does not change the semantics: the fatal log line must name the exact one-step remediation
  ("remove `<file>` from `AUTO_RESTORE_DIR`, or unset `AUTO_RESTORE_DIR`, then restart"). The
  crash-loop is recorded as an accepted consequence in the hazard table rather than mitigated.
- **`gosplit backup` refuses a database with pending migrations** rather than
  backing it up as-is. The plan only said not to migrate; refusing is the
  honest version, since an archive whose recorded migration set disagrees with
  its schema could never be restored anyway.
- **The manifest is written last in the archive, not first.** Its row and
  upload counts are then what was actually written rather than what a pre-pass
  predicted -- and it removes the plan's plaintext-temp-file hazard entirely,
  since nothing unsealed ever touches disk.
- **`RecoverIncompleteSwap` runs after `config.Load()` in `run()`**, not at the
  very top of `main()` as the plan said: it needs `cfg.UploadDir`. It is also
  called from the CLI restore, which the plan overlooked -- otherwise a CLI
  restore interrupted mid-swap stayed broken until someone started the server.
- **A janitor goroutine sweeps expired jobs and abandoned uploads**
  (`httpapp/janitor.go`), which the plan did not describe. The plan assumed the
  scheduled-backup prune would serve as the backstop; it would not, on two
  counts. The job tracker is per-process memory, so a leader-gated sweep would
  leave every replica but one leaking, and the scheduler's prune only runs when
  `BACKUP_CRON` is set, while uploads happen through the panel regardless. The
  `.partial` sweep moved there too, so it has a single owner.
- **Chunked AEAD instead of a single `Seal`.** The winning design sealed the whole `gzip(tar)`
  blob in one `Seal` call, which needs the entire archive resident in memory. With uploads
  included that ceiling is unbounded. This plan frames the payload into 1 MiB sealed chunks
  instead, holding memory flat. Roughly 60 extra lines, and it also makes truncation and
  reordering authentication failures rather than silent data loss.

## Archive format

One sealed file, extension `.gsbak`.

```
magic          "GSPLTBAK"   8 bytes
format_version uint8        = 1
header_len     uint32 BE
header          header_len bytes of plaintext JSON  <- readable without the key
payload         framed sealed chunks: repeated (uint32 BE sealed_len || sealed bytes)
```

The header is plaintext by design (R5): a restore identifies format, engine and key fingerprint
and fails fast before deriving a key or calling `Open`. Its exact bytes are part of every
chunk's AEAD associated data, so a header and a payload cannot be spliced from different
archives.

**Header fields:** `format_version`, `created_at`, `source_engine`, `gosplit_version`,
`kdf_algo` (`"scrypt"`), `kdf_n`, `kdf_r`, `kdf_p`, `kdf_salt_b64` (16 random bytes, fresh per
archive), `key_fingerprint_b64` (32 bytes), `aead_algo` (`"AES-256-GCM"`), `nonce_prefix_b64`
(7 random bytes, fresh per archive), `chunk_size` (1048576), `schema_migrations_hint` (sorted
filenames -- advisory, a fast pre-decrypt UX hint only; the authoritative check is always the
inner manifest after decrypt).

**Sealed payload** decrypts and gunzips to a tar with exactly these entries:

1. `manifest.json` -- the authoritative record.
2. `tables/<name>.jsonl`, one per table in `schema.InsertOrder()` (21 files), one JSON array per
   line, LF-terminated.
3. `uploads/<relative path under cfg.UploadDir>`, `TypeReg` only.

**manifest.json fields:**

- `format_version int` -- must equal the binary's `backup.FormatVersion`.
- `created_at string` -- ISO-8601 UTC, the same shape as `nowISO()`.
- `source_engine string`, `gosplit_version string` -- diagnostics only; restore works either
  direction.
- `schema_migrations []string` -- the **sorted set of migration filenames** applied at dump
  time. Deliberately carries **no `applied_at`**: that column records when a given database
  first saw a migration, not the migration's identity, so comparing it would make every restore
  onto a freshly provisioned database -- the primary disaster-recovery case -- fail with a false
  mismatch. Only the version-string set is ever compared.
- `tables []{name string; row_count int64}` -- one per table, in `InsertOrder()`. Enforced, not
  advisory.
- `upload_file_count int`, `upload_total_bytes int64` -- enforced the same way.

### Structural validation (all before any mutation)

Restore builds **one** canonical entry index, reused by both validate and apply -- never two
independent tar passes, so what was checked is exactly what gets applied. Building that index is
where every structural check lives:

- every name in `schema.InsertOrder()` must have **exactly one** `tables/<name>.jsonl` entry.
  **Missing is a hard failure** -- rejecting only unknown entries would let a crafted archive
  omit `tables/expense_participants.jsonl` and silently wipe every participant row, zeroing
  every balance in the instance.
- a second entry with the same name is a hard failure, so no duplicate silently wins.
- any `tables/*.jsonl` not matching a registry name (`tables/balance_view.jsonl`, say) is a hard
  failure.
- any `uploads/*` entry whose cleaned path escapes `UploadDir`, is absolute, or contains `:` or
  `\` is a hard failure. Tar names are always `/`-separated, so either character is itself
  anomalous -- and the predicate is OS-independent, so a CLI restore run from a Windows shell is
  covered identically to the Linux container.
- any tar `Typeflag` other than `TypeReg`/`TypeDir` (symlink, hardlink, device, fifo) is a hard
  failure.
- per-entry and cumulative byte caps are measured from **actual bytes read** via
  `io.LimitReader`, never from the tar or gzip header's claimed size -- that is what makes the
  decompression-bomb guard real.
- after decoding a table's JSONL, its line count must equal `manifest.tables[name].row_count`
  exactly.

A malformed archive is systemic, not a single bad row, so this fails the whole restore loudly
rather than skipping entries.

### Row encoding

Every `tables/<name>.jsonl` line is a **JSON array**, one element per column in `schema.go`'s
fixed order for that table -- **positional, not keyed by column name.** That is a second,
structural layer of injection defense on top of the compiled allowlist: no column name ever
appears inside archive content at all.

Per-element encoding is chosen by the column's declared kind in `schema.go`, never by sniffing
the archive:

| Kind | Wire form | Why |
|---|---|---|
| `KindInt64` | a **quoted** JSON string of the decimal value, e.g. `"9223372036854775807"`; decoded with `strconv.ParseInt`. A bare JSON number in this slot is a decode error | `encoding/json` decodes numbers to `float64`, losing precision above 2^53. Every money column is `int64` minor units |
| `KindBool` | JSON `true`/`false`, engine-independent on the wire. Binding is the one place that branches on engine: SQLite binds `0`/`1` (physical `INTEGER`), Postgres binds a Go `bool` (`BOOLEAN`) | `groups.simplify_debts` and `expense_recurrences.notified` diverge between dialects |
| `KindText` / `KindNullText` | JSON string, or `null` | `null` means SQL NULL and is distinct from `""` |
| `KindJSONText` | JSON string carrying the stored text **verbatim**, never parsed and re-serialized | `hidden_friend_ids`, `shares`, `expense_split_inputs.inputs`, `cached_bank_data.data`, `push_notifications.subscription` must survive byte-for-byte, key order included |

Nothing is ever scanned into a bare `any` -- that yields `[]byte` on one driver and `string` on
the other, which is precisely how a cross-engine dump corrupts itself.

### Encryption

- **Master key:** `scrypt(SESSION_SECRET, kdf_salt, N, r, p)`. scrypt rather than bare HKDF
  because nothing enforces that `SESSION_SECRET` is *random* -- `config.go:137` only checks
  length >= 16, so a low-entropy passphrase is legal and a slow KDF is warranted.
- **Subkeys:** `sealKey, fingerprintKey = HKDF-SHA256(master)` with distinct info labels, so the
  fingerprint can be published without weakening the sealing key.
- **Fingerprint:** `HMAC-SHA256(fingerprintKey, fixed label)`, 32 bytes, in the plaintext header,
  compared with `hmac.Equal` **before** any `Open` call. A wrong secret therefore yields
  `ErrWrongSessionSecret` ("archive was sealed with a different SESSION_SECRET") rather than a
  bare `cipher: message authentication failed`.
- **Sealing:** AES-256-GCM over 1 MiB plaintext chunks.
  `nonce = nonce_prefix(7) || counter uint32 BE || last_flag(1)` = 12 bytes, the standard GCM
  length. The counter increments per chunk; `last_flag` is `0x01` only on the final chunk, so a
  truncated, reordered or duplicated stream is an authentication failure, not partial data. AAD
  per chunk is the exact plaintext header bytes.
- The plaintext `gzip(tar)` staging blob contains `users.password_hash`, `sessions.token`,
  `verification_tokens.token` and `cached_bank_data.data` verbatim. It is created `0600` and
  removed via `defer` registered immediately after the handle is obtained, so every return path
  is covered, not just success.

**Required warning** for the README and `.env.example`: the archive is sealed with a key derived
from `SESSION_SECRET`. Rotating or losing that secret makes every existing archive permanently
unrestorable. There is no override key, by decision (R5).

## Implementation

### Step 1 -- `internal/store/store.go`

- `func (s *Store) Rebind(query string) string { return s.rebind(query) }` -- export the existing
  `?`-to-`$n` rewriter ([store.go:110-126](internal/store/store.go#L110-L126)) so
  `internal/backup` builds one portable SQL string per statement instead of reimplementing the
  rule. Mechanical, zero behavior change.
- `func OpenNoMigrate(ctx context.Context, cfg *config.Config) (*Store, error)` -- identical to
  `Open` except it never calls `s.migrate(ctx)` ([store.go:63](internal/store/store.go#L63)).
  **Why this matters:** `Open` migrates unconditionally, so a bare `gosplit backup` -- a separate
  OS process against a still-live production database, typically run right *before* a version
  upgrade -- would silently apply every pending migration, including a future blanket data
  migration like `0005`, as a side effect of taking a backup, with no confirmation. Extract the
  shared connect/ping body so both entry points use it; the server boot path is unchanged.
  A never-initialized volume gives `OpenNoMigrate` callers an actionable "database not
  initialized -- start the server once first" error from the first failing query.

### Step 2 -- `internal/config/config.go`

Additive fields in the existing `getEnv*` style ([config.go:83-117](internal/config/config.go#L83-L117)):

```
BackupDir                string        // BACKUP_DIR, default ./data/backups
BackupCron               string        // BACKUP_CRON, default "" (disabled)
BackupRetentionCount     int           // BACKUP_RETENTION_COUNT, default 7 (<=0 keeps all)
AutoRestoreDir           string        // AUTO_RESTORE_DIR, default "" (disabled)
RestoreMaxUploadMB       int           // RESTORE_MAX_UPLOAD_MB, default 500
RestoreMaxArchiveBytes   int64         // RESTORE_MAX_ARCHIVE_BYTES, default 2 GiB
RestoreMaxEntries        int           // RESTORE_MAX_ENTRIES, default 200000
RestoreMaxTableFileBytes int64         // RESTORE_MAX_TABLE_FILE_BYTES, default 512 MiB
RestoreMaxJSONLLineBytes int           // RESTORE_MAX_JSONL_LINE_BYTES, default 8 MiB
AppVersion               string        // NOT from the environment -- main assigns the link-time stamp
```

`Load()` fails loud, matching the `SESSION_SECRET` precedent at
[config.go:134-139](internal/config/config.go#L134-L139), when:

- `BackupCron` is non-empty and does not parse;
- `BackupCron` is set but `BackupDir` is empty;
- `BackupDir != "" && BackupDir == AutoRestoreDir` -- otherwise the server would restore its own
  most recent scheduled backup on the next boot.

Cron parsing reuses the shape already in the codebase at
[recurrence_service.go:17](internal/service/recurrence_service.go#L17):
`cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)`.
`github.com/robfig/cron/v3` is already a direct dependency and already used there, so R8 needs
no new dependency and no new idiom.

Comment `AppVersion` to say it is set by `main` from the link-time stamp, not read from env.

### Step 3 -- `internal/backup/schema.go`

New package, depending only on `internal/store`, `internal/config` and stdlib plus
`golang.org/x/crypto`. **This file is the security boundary** -- the only place table or column
identifiers are ever Go string literals feeding SQL text. Archive content is only ever a
map-lookup key or a bound `?` value.

```go
type ColumnKind int // KindInt64, KindBool, KindText, KindNullText, KindJSONText
type Column struct{ Name string; Kind ColumnKind }
type Table  struct{ Name string; Columns []Column; AutoIncrement bool }

var Tables []Table // 21 tables, FK-safe topological order

func InsertOrder() []Table
func DeleteOrder() []Table              // exact reverse of InsertOrder()
func LookupTable(name string) (Table, bool)
func PostgresSequenceTables() []string  // users, groups, expense_notes, expense_recurrences
```

Order: `users`, `groups`, `group_users`, `friendships`, `group_default_splits`,
`friend_default_splits`, `expenses`, `expense_participants`, `expense_split_inputs`,
`expense_notes`, `expense_recurrences`, `sessions`, `verification_tokens`, `push_notifications`,
`cached_bank_data`, `cached_currency_rates`, `scheduler_locks`, `app_metadata`,
`archived_expenses`, `archived_expense_participants`, `schema_migrations`.

Columns and kinds are transcribed from
`internal/store/migrations/{sqlite,postgres}/0001..0006.sql`, including the four orphaned tables
(`group_default_splits`, `friend_default_splits`, `expense_notes`, `app_metadata`) and the two
write-only ones. Comment the latter explicitly: `archived_expenses` and
`archived_expense_participants` have no Go SELECT path and no struct -- their only existing code
path is the INSERTs at [expenses.go:490](internal/store/expenses.go#L490) and
[:501](internal/store/expenses.go#L501) -- so any dump built from typed store methods loses the
entire archive silently. Generic column-kind scanning captures them for free.

`balance_view` is excluded by never appearing in the list. It is derived, and restoring it would
create a second source of truth for money.

Strict topological order is used on both engines rather than deferring constraint checks:
Postgres `SET CONSTRAINTS ALL DEFERRED` only affects `DEFERRABLE` constraints and the schema
declares plain `REFERENCES`; SQLite's `PRAGMA foreign_keys` is a no-op inside a transaction and
the DSN pins it on at [store.go:92](internal/store/store.go#L92). Ordering is the only mechanism
that behaves identically on both.

### Step 4 -- `internal/backup/codec.go`

Pure, DB-free value normalization shared by dump and restore, parameterized by
`(config.Engine, ColumnKind)`. This is where R7's engine-agnostic tests run, with no live DB.

```go
func ScanDest(engine config.Engine, kind ColumnKind) any
func EncodeColumn(engine config.Engine, kind ColumnKind, scanned any) (json.RawMessage, error)
func DecodeColumn(engine config.Engine, kind ColumnKind, raw json.RawMessage) (bindArg any, err error)
```

### Step 5 -- `internal/backup/crypto.go`

```go
func DeriveMasterKey(sessionSecret string, salt []byte) ([]byte, error)      // scrypt
func DeriveSubkeys(master []byte) (sealKey, fingerprintKey [32]byte, err error) // HKDF-SHA256
func Fingerprint(fingerprintKey []byte) []byte                              // HMAC-SHA256
func NewSealWriter(w io.Writer, sealKey, noncePrefix, aad []byte) io.WriteCloser
func NewOpenReader(r io.Reader, sealKey, noncePrefix, aad []byte) io.Reader
var ErrWrongSessionSecret = errors.New("backup: archive was sealed with a different SESSION_SECRET")
```

`Close` flushes the final chunk with `last_flag = 0x01`. The open reader errors if the stream
ends without a final-flagged chunk.

### Step 6 -- `internal/backup/archive.go`

```go
type Header struct{ /* the outer fields above */ }
func WriteArchive(w io.Writer, plaintextTarGzPath string, hdr Header, sealKey []byte) error
func ReadHeader(r io.Reader) (Header, io.Reader, error)   // no key needed
func DecryptBody(hdr Header, body io.Reader, sealKey []byte) (tarGzPath string, err error)

type Limits struct {
    MaxTotalBytes, MaxUploadFileBytes, MaxTableFileBytes int64
    MaxEntries        int
    MaxJSONLLineBytes int
}
type Entry struct{ Name string; Kind EntryKind; Header *tar.Header }

func IndexEntries(tarGzPath string, limits Limits) (map[string]Entry, error)
func StreamTableRows(tarGzPath string, e Entry, maxLineBytes int) iter.Seq2[[]json.RawMessage, error]
func ExtractUploads(tarGzPath string, entries map[string]Entry, stagingDir string) error
```

`IndexEntries` is the single pass enforcing every rule in the "Structural validation" section
above; both `ValidateArchive` and `ApplyRestore` consume its output rather than re-deriving an
index. `StreamTableRows` uses a `bufio.Scanner` with an explicit buffer capped at
`maxLineBytes`, so an over-long line is a named "table row exceeds
RESTORE_MAX_JSONL_LINE_BYTES" error rather than an opaque `bufio.ErrTooLong`.

### Step 7 -- `internal/backup/dump.go`

```go
func Dump(ctx context.Context, st *store.Store, cfg *config.Config, tarGzOut io.Writer) (Manifest, error)
```

**A consistent snapshot read is mandatory, not an optimization.** `UpdateExpense`
([expenses.go:318](internal/store/expenses.go#L318)) implements a split edit as
`DELETE FROM expense_participants WHERE expense_id = ?` followed by re-INSERTs. A paginated dump
that reads `expenses` and `expense_participants` in separate transactions can capture the delete
but not the re-insert, producing an archive whose participant rows no longer sum to zero -- a
backup that silently corrupts money on restore. Snapshot strategy, per engine:

- **SQLite:** open a **separate** `*sql.DB` directly against the same DSN with `mode=ro`,
  bypassing the server's pool entirely. WAL is already enabled
  ([store.go:89-92](internal/store/store.go#L89-L92)), so a reader on its own connection does
  not block the writer -- which matters because the server's pool is pinned to one connection
  ([store.go:48](internal/store/store.go#L48)) and a long-lived transaction on it would stall
  every HTTP handler for the whole dump. Begin one `BEGIN DEFERRED` read transaction spanning
  the dump; paginate inside it only to bound memory, never to release the connection.
- **Postgres:** the normal pool, `st.DB.BeginTx(ctx, &sql.TxOptions{Isolation:
  sql.LevelRepeatableRead, ReadOnly: true})`.

Uploads are walked after the table snapshot closes. They are not covered by the DB transaction;
this is an accepted small window, since an upload file is immutable once written.

`Manifest.tables[name].row_count` is what this dump actually wrote, and is enforced on restore.

`WriteFile` names the archive `gosplit-backup-<UTC timestamp>.gsbak`, writes to a `.partial`
name in the destination directory and renames on success, so no reader -- including the
auto-restore scan -- ever sees a half-written archive.

### Step 8 -- `internal/backup/restore.go`

Validate and apply are split so the admin panel can preview before the irreversible step.

```go
type Validated struct {
    Manifest  Manifest
    Entries   map[string]Entry
    TarGzPath, StagingUploadsDir string
}
var ErrSchemaMismatch = errors.New("backup: archive schema_migrations does not match this database")

func ValidateArchive(ctx context.Context, st *store.Store, cfg *config.Config, path string) (Validated, error)
func ApplyRestore(ctx context.Context, st *store.Store, cfg *config.Config, v Validated) (RestoreReport, error)
func SafetyDump(ctx context.Context, st *store.Store, cfg *config.Config) (path string, err error)
```

`ValidateArchive` -- **zero DB or filesystem mutation on every path:**

1. `ReadHeader`, recompute the fingerprint, `hmac.Equal`; `ErrWrongSessionSecret` on mismatch,
   **before** calling `Open`.
2. `DecryptBody`, then `IndexEntries` -- the one canonical pass.
3. Decode `manifest.json`; require `format_version == backup.FormatVersion`.
4. Compare `manifest.schema_migrations` (**version strings only, never `applied_at`**) against
   `SELECT version FROM schema_migrations`. Any difference -- missing, extra or unrecognized --
   returns `ErrSchemaMismatch` naming the exact differing filenames. An exact-match requirement
   is what categorically closes the `0005_simplify_debts_default.sql` hazard: that migration is a
   blanket `UPDATE groups SET simplify_debts = 1`, so a restore that left the recorded set
   inconsistent with the live schema would let it re-fire on a later boot and clobber every
   group's flag. Refusing is simpler and safer than reconciling.
5. `ExtractUploads` into a fresh staging directory under `cfg.BackupDir`, fully hardened. The
   live `UploadDir` is still untouched.

`ApplyRestore`:

1. `SafetyDump` of the **current** data. If it fails, abort before touching anything -- this is
   the operator's only way back, and it composes for free from step 7.
2. Set the maintenance flag (step 14) when running in-process.
3. One `*sql.Tx`:
   - **Postgres:** `LOCK TABLE <all 21> IN ACCESS EXCLUSIVE MODE` first, so any concurrent
     writer -- this process or another -- blocks at the engine level for the transaction rather
     than racing the delete/insert or colliding with a not-yet-reset sequence.
   - **SQLite:** this transaction's own handle uses a 5-minute `busy_timeout` (versus the
     server's 5000 ms at [store.go:90](internal/store/store.go#L90)), so a writer from the live
     server process blocks and retries instead of failing fast with `SQLITE_BUSY`.
   - `DeleteOrder()`: `DELETE FROM <table>` for every table. SQL text built exclusively from
     `schema.go`.
   - `InsertOrder()`: decode each JSONL row via `DecodeColumn` and bind positionally, batching
     multi-row `VALUES` at `floor(65535 / len(cols))` to stay under Postgres's parameter cap.
   - **Postgres only:** for each `PostgresSequenceTables()` entry,
     `SELECT setval(pg_get_serial_sequence(<table>, 'id'), COALESCE(MAX(id), 0) + 1, false)`.
     Without this the next insert collides with a restored id.
   - Verify each table's inserted count against `manifest.tables[name].row_count`; a mismatch
     rolls back with an error naming the table.
   - `COMMIT`.
4. Two-rename uploads swap: `os.Rename(UploadDir, UploadDir+".old")`, then
   `os.Rename(StagingUploadsDir, UploadDir)` -- both same-filesystem, since `BackupDir` and
   `UploadDir` share `/data`. If the second fails, rename `.old` back. On success,
   `os.RemoveAll(UploadDir+".old")`.
5. Clear the maintenance flag via `defer`, on every exit path.

### Step 9 -- `internal/backup/recovery.go`

The uploads swap cannot join the SQL transaction, so a crash mid-swap needs boot-time repair.
Runs at the very top of `main()`, before either subcommand or `run()`.

```go
func RecoverIncompleteSwap(cfg *config.Config) error
```

- Pre-commit crash (only a staging directory present): delete it, nothing else to fix.
- Post-commit, pre-swap crash (DB committed, staging still valid): safe to leave; the next
  restore attempt redoes the swap.
- Mid-swap crash (`UploadDir+".old"` present **and** `UploadDir` missing): rename `.old` back and
  return a loud, actionable "uploads/DB inconsistency, the restore may need re-running" error
  that aborts boot.

### Step 10 -- CLI subcommands

New `cmd/gosplit/backup_cli.go`; `cmd/gosplit/main.go` modified. Generalize the `os.Args[1]`
switch without touching `wantsVersion`'s body, so `main_test.go`'s existing
`{"gosplit","serve"}` and `{"gosplit","serve","--version"}` cases keep passing unchanged.

```go
func wantsBackup(args []string) (cmd string, ok bool) // "backup" | "restore" | "inspect"
func runBackupCLI(args []string) error
func runRestoreCLI(args []string) error
func runInspectCLI(args []string) error
```

- `gosplit backup [-o <path>]` -- defaults to a generated name under `cfg.BackupDir`. Uses
  `store.OpenNoMigrate`.
- `gosplit restore --file <path> [--force]` -- without `--force`, runs `ValidateArchive` only,
  prints the manifest summary and refuses, touching nothing (`OpenNoMigrate`). With `--force`,
  opens normally -- the insert needs the migrated schema, and `ValidateArchive`'s exact-match
  schema check is the real gate -- then `ApplyRestore`.
- `gosplit inspect <path>` -- prints the plaintext header without needing a matching secret. The
  natural first thing an operator reaches for.

`flag.NewFlagSet` per subcommand. Status and progress go to stderr, never through the
mailer/service stack, keeping the dispatch minimal. Failures exit non-zero with the wrapped
cause.

In the container: `docker exec <ctr> /app/gosplit backup -o /data/backups`. The distroless
runtime has no shell, but `ENTRYPOINT` is exec-form and the binary is at `/app/gosplit`, so a
direct exec works.

### Step 11 -- Startup auto-restore

`internal/backup/autorestore.go`, wired into `run()` immediately after `store.Open`'s
"database ready" log ([main.go:89](cmd/gosplit/main.go#L89)) and before `mail.New`
([main.go:91](cmd/gosplit/main.go#L91)). That is the only safe window: the schema is migrated
and `cfg.Engine` is known, but no listener and no scheduler exist yet, so nothing can write
concurrently.

```go
func CheckAutoRestore(ctx context.Context, st *store.Store, cfg *config.Config) error
```

1. No-op when `cfg.AutoRestoreDir == ""`.
2. No-op when the marker file is present (bare presence check, per R6).
3. Find the archive: exactly one `*.gsbak`, ignoring `safety-*.gsbak` and `*.partial`. Zero ->
   skip. More than one -> fatal; an ambiguous restore must never be guessed at.
4. **Probe that the marker can be written** -- create and immediately remove a temp file in
   `AutoRestoreDir`. If the directory is read-only, abort now: without this probe a
   bare-presence design silently re-restores on every restart forever.
5. Acquire the `"auto-restore"` lock via `st.AcquireLock`
   ([locks.go:14-33](internal/store/locks.go#L14-L33)), then start a **renewal goroutine** that
   re-acquires every `ttl/3` for as long as validate and apply run, stopped by `defer`. A single
   acquire with a fixed TTL that a slow restore outlives would let a second replica double-fire
   the wipe. This mirrors how `scheduler.tick()` keeps `"cleanup"` fresh by re-acquiring each
   tick. A replica that loses the lock boots normally without restoring.
6. `ValidateArchive`, then `ApplyRestore`.
7. Write the marker only after both the DB commit and the uploads swap succeed.

**Every failure is fatal and aborts boot (R9), schema-version mismatch included.** The error is
returned from `run()`, which `main` logs and exits 1; the transaction has already rolled back, so
the database is untouched. A failed marker write after an otherwise successful restore is also
fatal -- a silently absent marker means the next restart repeats the destructive restore against
whatever has accumulated since.

Because a mismatched or corrupt archive left in the folder therefore crash-loops the container,
every fatal message here must name the one-step remediation: `remove <file> from
AUTO_RESTORE_DIR, or unset AUTO_RESTORE_DIR, then restart`. See Deviations.

### Step 12 -- Scheduled backup

`internal/scheduler/scheduler.go` modified, plus `internal/backup/prune.go`.

Add `sched cron.Schedule` and `nextAt time.Time` to the `Scheduler` struct
([scheduler.go:18-22](internal/scheduler/scheduler.go#L18-L22)), parsed from `cfg.BackupCron` in
`New` (config already validated it; assert anyway).

The due-check runs inside the existing `tick()`, but the dump does **not** run inline -- a slow
backup must not delay recurrence generation or session/token/rate cleanup, nor risk its lock
lapsing mid-dump. When due, launch a goroutine with its own context and its own
`"scheduled-backup"` lock plus the same renewal loop as step 11; `tick()` returns immediately.
The existing `"cleanup"` lock has a `5*Interval` = 5 minute TTL
([scheduler.go:48](internal/scheduler/scheduler.go#L48)), which a real backup would outlive, so
reusing it is not an option.

On success: `PruneOldArchives(cfg.BackupDir, pattern, cfg.BackupRetentionCount)`, then advance
`nextAt = sched.Next(now)`. `nextAt` is in-memory; a window missed across a restart is not a
correctness problem, and the lock is what prevents a double fire.

```go
func PruneOldArchives(dir, filenamePattern string, keep int) error
```

Shared with safety-dump retention and with the abandoned restore-upload temp-file sweep.

### Step 13 -- Admin panel

New: `internal/backup/jobs.go`, `internal/httpapp/handlers_backup.go`,
`internal/web/templates/admin_backup.html`. Modified: `internal/httpapp/server.go`,
`internal/web/view.go`, `internal/web/templates/admin.html`, `internal/i18n/locales/*.json`.

**Backup and restore must be asynchronous.** `http.Server.WriteTimeout` is 60s
([main.go:113](cmd/gosplit/main.go#L113)), `ReadTimeout` 30s, and handlers use a 10s `ctxTimeout`
([server.go:349-351](internal/httpapp/server.go#L349-L351)). None of those can hold a real
backup or restore, and raising them globally would weaken every other route.

```go
// internal/backup/jobs.go -- per-Server, never a package global.
type JobTracker struct{ mu sync.Mutex; jobs map[string]*JobStatus }
type JobStatus struct {
    State               string // "running" | "done" | "error"
    Err                 string // sanitized -- see the hazard table
    ResultPath          string
    RestoredAdminUserID sql.NullInt64
    CreatedAt           time.Time
}
func NewJobTracker() *JobTracker
func (t *JobTracker) Start(fn func(ctx context.Context) (JobStatus, error)) (jobToken string)
func (t *JobTracker) Status(jobToken string) (JobStatus, bool)
func (t *JobTracker) Prune(olderThan time.Duration)
```

`Start` roots the job in `context.Background()`, never `r.Context()`, which is cancelled when the
kickoff response returns.

```go
// internal/httpapp/handlers_backup.go
func (s *Server) handleBackupPage(w http.ResponseWriter, r *http.Request)
func (s *Server) handleBackupGenerate(w http.ResponseWriter, r *http.Request)
func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request)
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request)
func (s *Server) handleRestoreUpload(w http.ResponseWriter, r *http.Request)
func (s *Server) handleRestoreConfirm(w http.ResponseWriter, r *http.Request)
func (s *Server) handleRestoreJobStatus(w http.ResponseWriter, r *http.Request)
```

Routing in `server.go`. `GET/POST /admin/backup*` and `/admin/restore/confirm` sit inside the
existing `RequireUser + RequireAdmin` group
([server.go:146-154](internal/httpapp/server.go#L146-L154)). Two routes need deliberate
exceptions:

- **`POST /admin/restore/upload` is pulled out of the broad `VerifyCSRF` group** and registered
  with its own order:
  `r.With(maxBytesMiddleware(cfg.RestoreMaxUploadMB), s.Auth.RequireUser, s.Auth.RequireAdmin,
  s.Auth.VerifyCSRF).Post(...)`.
  `VerifyCSRF` falls back to `r.FormValue("csrf_token")`
  ([middleware.go:88-110](internal/auth/middleware.go#L88-L110)), and `FormValue` calls
  `ParseMultipartForm` -- so on the normal ordering the CSRF middleware fully buffers an
  unbounded multipart body, spilling to `/tmp`, *before* the handler's `MaxBytesReader` ever
  runs. Wrapping `r.Body` first is what makes the cap real. The raw bytes then go straight to a
  temp file under `cfg.BackupDir` -- never `/tmp` or `/app`, which are not writable on the
  distroless nonroot rootfs.
- **`GET /admin/restore/status/{jobToken}` is registered outside the `RequireUser` group**, gated
  only by the high-entropy job token and a 10-minute TTL. This is deliberate: a restore replaces
  `users`, and `sessions` CASCADE-deletes with it, so the acting admin's session row is gone and
  the cookie the browser still holds cannot authorize the poll that observes completion. When
  the handler sees `State == "done"`, it looks up `RestoredAdminUserID` in the just-restored
  `users` table -- by id, never by trusting the archive -- and if the row exists calls
  `auth.Manager.SetSession(w, r, userID)` in that same response. A background goroutine has no
  `ResponseWriter` and cannot do this. If the row is gone, it clears the cookie and shows a
  plain "log in again" message.

`handleBackupDownload` validates the filename against `^gosplit-backup-[0-9TZ-]+\.gsbak$`, then
`filepath.Clean` and a prefix check against `cfg.BackupDir` before opening -- otherwise
download-by-filename is arbitrary file disclosure, the live SQLite database included.

Template `admin_backup.html` defines `{{define "content"}}` and follows `admin.html`'s idioms: a
`.m-head` header, a `.tiles`/`.tile` row, a `.card` per action, and a warning card above the
restore form. **Add `"admin_backup": "admin_backup.html"` to the `pageFiles` map at
[view.go:142-164](internal/web/view.go#L142-L164)** -- a page absent from that map is never
parsed. Link it from `admin.html`'s `.m-head`. The download control carries `hx-boost="false"`;
a boosted anchor swallows `Content-Disposition`, which is why `/profile/export` carries it at
[profile.html:57](internal/web/templates/profile.html#L57), documented by
[progressive_test.go:85](internal/httpapp/progressive_test.go#L85). The restore form is a
two-step reveal: upload -> preview the manifest summary from `ValidateArchive` -> typed-phrase
confirm. The phrase is the fixed literal `RESTORE ALL DATA`, checked server-side and
deliberately not localized, so it is unambiguous in every locale. New `nav.backup` /
`admin.backup_*` keys go into all 9 locales.

### Step 14 -- Maintenance-mode write gate

`internal/httpapp/middleware_maintenance.go` (new), `server.go` modified. A
`Server.restoreInProgress atomic.Bool`, set immediately before `ApplyRestore`'s transaction
opens and cleared via `defer`, checked by a small middleware early in the chain. While set, any
non-GET request other than the restore-status poll gets `503` with `Retry-After` and an
actionable i18n'd message -- instead of queueing behind SQLite's sole connection until an
unexplained 10s `ctxTimeout` fires.

This covers only the in-process admin-panel trigger. A CLI restore runs in a separate OS process
with no access to the flag, so on SQLite it is documented (README and the `--force` output) as a
maintenance-window operation, backstopped by the 5-minute `busy_timeout` from step 8. On
Postgres the `LOCK TABLE ... ACCESS EXCLUSIVE` handles both triggers uniformly at the engine
level.

### Step 15 -- Version in the UI (R10)

- `main.go`: `cfg.AppVersion = version` immediately after `config.Load()`.
- `internal/web/view.go`: add `Version string` to `ViewData`
  ([view.go:198-209](internal/web/view.go#L198-L209)).
- `internal/httpapp/server.go`: set it in `s.vd`
  ([server.go:162-179](internal/httpapp/server.go#L162-L179)) from `s.Cfg.AppVersion`, so both
  `vd` and `vdPage` carry it.
- `layout.html:58-60`: append `<span class="ver">{{.Version}}</span>` inside `.site-footer`,
  beside the existing GitHub link.
- `handleHealth` ([server.go:283-290](internal/httpapp/server.go#L283-L290)): build the body with
  `encoding/json` instead of the current literal, emitting `{"status":"ok","version":"..."}`.

## Hazards and mitigations

| Hazard | Severity | Mitigation | Where enforced |
|---|---|---|---|
| SQL identifier injection from archive-controlled table or column names | Critical | `schema.go` is the only source of identifiers in SQL text; archive strings are only map keys or bound `?` values. Rows are positional arrays, so column names never appear in archive content at all | `schema.go`, `restore.go` builders |
| A missing table entry silently wipes that table -- omit `expense_participants` and every balance zeroes | Critical | `IndexEntries` requires exactly one `tables/<name>.jsonl` per `InsertOrder()` name; missing is a hard failure before any mutation | `archive.go: IndexEntries` |
| Duplicate `tables/<name>.jsonl`: one occurrence validated, another applied | High | One canonical index built once and reused by validate and apply; a second same-name entry is a hard failure | `archive.go: IndexEntries`, `restore.go` |
| Live paginated dump tears the `expense_participants` zero-sum invariant across a concurrent split edit -- a backup that corrupts money | Critical | One consistent-snapshot read per engine: SQLite via a dedicated `mode=ro` connection outside the pool with one `BEGIN DEFERRED` spanning the dump; Postgres via `REPEATABLE READ, ReadOnly` | `dump.go: Dump` |
| CLI `backup` silently fires pending migrations against live production via `Open`'s unconditional `migrate()` | Critical | `store.OpenNoMigrate`; CLI backup and the restore preview path use it | `store.go`, `backup_cli.go` |
| CSRF middleware's `FormValue` fallback buffers the whole multipart restore upload before `MaxBytesReader` runs, defeating the cap and spilling to `/tmp` | Critical | Route-scoped `maxBytesMiddleware` wraps `r.Body` **before** `VerifyCSRF` in that route's own chain | `server.go` route registration |
| Auto-restore's fixed-TTL lock lapses mid-restore, letting a second replica double-fire the wipe | Critical | Renewal goroutine re-acquires every `ttl/3` for the duration of validate and apply; same pattern for `"scheduled-backup"` | `autorestore.go`, `scheduler.go` |
| `ApplyRestore`'s transaction holds SQLite's sole connection, stalling every request | Critical | Admin-panel trigger sets a maintenance flag that fast-fails mutating requests with 503; the restore handle uses a 5-minute `busy_timeout` so a racing writer blocks and retries; CLI restores on SQLite documented as a maintenance window | `middleware_maintenance.go`, `restore.go`, README |
| `schema_migrations` compared including `applied_at` blocks every restore onto a fresh database -- the primary DR case | Critical | Compare only the sorted version-string set; `applied_at` is never part of it | `restore.go: ValidateArchive` |
| `0005`-style blanket data migration re-fires after a restore, clobbering every group's `simplify_debts` | Critical | Exact-match refusal on the version-string set closes this categorically, for `0005` and any future blanket migration | `restore.go: ValidateArchive` |
| int64 money loses precision through `encoding/json`'s `float64` decode | High | `KindInt64` is always a quoted JSON string decoded with `strconv.ParseInt`; a bare number is a decode error. Tested at `math.MaxInt64` | `codec.go` |
| Cross-engine bool divergence (`INTEGER` 0/1 vs `BOOLEAN`) | High | `KindBool` is the only kind whose bind branches on engine; the wire form is always JSON `true`/`false` | `codec.go` |
| Postgres sequences stale after an ID-preserving restore, so the next insert collides | High | `setval(pg_get_serial_sequence(...), MAX(id)+1, false)` for the four autoincrement tables, in the same transaction | `restore.go` step 3 |
| `archived_expenses` / `archived_expense_participants` silently dropped -- no Go SELECT path exists | High | Generic column-kind scanning never goes through Go structs; both are ordinary registry entries with a dedicated round-trip test | `schema.go`, `dump_test.go` |
| `balance_view` dumped or restored, creating a second source of truth for money | High | Absent from the registry; a test asserts it stays absent | `schema.go`, `schema_test.go` |
| Tar path traversal, absolute paths, symlinks into `uploads/` | High | Only `TypeReg`/`TypeDir` accepted; reject any path that `Clean`s outside `UploadDir`, is absolute, or contains `:` or `\` -- OS-independent, so a Windows-hosted CLI restore is covered identically | `archive.go: IndexEntries` |
| Decompression bombs and entry floods | High | Every entry and the cumulative stream counted from actual bytes read via `io.LimitReader`, never declared sizes, against `MaxTotalBytes` / `MaxEntries` | `archive.go: IndexEntries` |
| No caps on `tables/*.jsonl`: memory exhaustion, or `bufio.ErrTooLong` on a legitimately large JSON-TEXT row | High | `MaxTableFileBytes` and `MaxJSONLLineBytes` on a bounded `bufio.Scanner`, with a named error | `archive.go` |
| Concurrent Postgres writers race the delete/insert or a not-yet-reset sequence | High | `LOCK TABLE <all 21> IN ACCESS EXCLUSIVE MODE` at the start of the transaction, covering both triggers | `restore.go` step 3 |
| Post-restore session re-auth is impossible from a background goroutine, and the poll behind `RequireUser` is rejected the instant sessions are wiped | High | Status poll is token-gated with a short TTL, not session-gated; on completion it looks up `RestoredAdminUserID` by id and mints or clears the session in that same live response | `handlers_backup.go`, `jobs.go` |
| Download-by-filename is arbitrary file disclosure, including the live SQLite DB | High | Fixed-regex filename check, then `Clean` + prefix check against `cfg.BackupDir` | `handlers_backup.go` |
| A restore destroys data with no way back | High | Mandatory pre-restore `SafetyDump`, aborting if it fails; `--force` on the CLI; typed `RESTORE ALL DATA` in the panel | `restore.go` step 1 |
| Wrong `SESSION_SECRET` surfaces an opaque AEAD error | High | Non-secret HMAC fingerprint in the plaintext header, `hmac.Equal`-compared before any `Open` | `crypto.go`, `restore.go` |
| Truncated, reordered or duplicated sealed stream read as valid partial data | High | Per-chunk counter and final-chunk flag inside the nonce, header bytes as AAD; a missing final flag is an error | `crypto.go` |
| Startup auto-restore loops forever when the watch directory is read-only | High | Probe marker writability before restoring; abort boot if it fails | `autorestore.go` |
| **A mismatched or corrupt archive left in `AUTO_RESTORE_DIR` crash-loops the container** | High | **Accepted, by decision (R9).** Not mitigated. Every fatal message names the one-step remediation: remove the file, or unset `AUTO_RESTORE_DIR`, then restart. See Deviations | `autorestore.go` |
| Scheduled backup dir doubling as the auto-restore dir: the server restores its own last backup on boot | High | `Load()` refuses `BackupDir == AutoRestoreDir` | `config.go` |
| Uploads rewrite cannot join the SQL transaction; a mid-swap crash leaves DB and files inconsistent | High | Stage, commit, then two same-filesystem renames with rollback; `RecoverIncompleteSwap` repairs a mid-swap crash at boot and aborts loudly | `restore.go` steps 4, `recovery.go` |
| Plaintext pre-encryption blob (password hashes, session tokens, bank data) left on disk on an error path | Medium | Created `0600`; removal deferred immediately after the handle is obtained, covering every return path | `archive.go: WriteArchive` |
| Async job errors leak a Postgres DSN password or a constraint `DETAIL` echoing a live row value | Medium | A shared `sanitizeErr` strips DSN credentials and generalizes constraint detail before anything reaches `JobStatus.Err` or the log | `jobs.go` |
| `.old` uploads directory never cleaned up -- disk exhaustion over repeated restores | Medium | `os.RemoveAll` immediately after a successful swap | `restore.go` step 4 |
| Abandoned restore-upload temp files accumulate on `/data` | Medium | Deleted on any `ValidateArchive` failure, and on job-TTL expiry by a per-process janitor goroutine. NOT leader-gated and not tied to `BACKUP_CRON`: the job tracker is per-process memory, so every instance must sweep its own | `restore.go`, `jobs.go`, `httpapp/janitor.go` |
| Scheduled backup inline in `tick()` blocks cleanup and risks its lock lapsing | Medium | Runs on its own goroutine with its own renewed lock; `tick()` never blocks on it | `scheduler.go` |
| Reading a half-written archive | Medium | Write `.partial` and rename; scans skip `.partial` | `dump.go`, `autorestore.go` |
| Multiple archives in the watch directory | Medium | Fail loud rather than guess | `autorestore.go` |
| Archive downloaded over plain HTTP when `BASE_URL` is not `https://` | Medium | `cfg.SecureCookies` already tracks this ([config.go:131](internal/config/config.go#L131)); show a warning on the backup page when false | `admin_backup.html` |
| `scheduler_locks` restored verbatim can carry a future-dated lock held by a dead holder | Low | Self-heals within `5*Interval` (5 minutes) because `AcquireLock` overwrites an expired row. Documented, not worked around, since R2 requires verbatim | README |

## Tests

Stdlib `testing` only, table-driven, `t.Helper`, `t.TempDir`, co-located, reusing `openTestStore`
([golden_test.go:15-26](internal/store/golden_test.go#L15-L26)), `newHarness`
([http_test.go:22-133](internal/httpapp/http_test.go#L22-L133)) and `newTestService`
([service_test.go:30-46](internal/service/service_test.go#L30-L46)).

`internal/backup/schema_test.go`
- `TestLookupTableRejectsUnknownNames` -- allowed names ok; adversarial names
  (injection-shaped, `balance_view`, traversal-shaped, empty) all `ok=false`.
- `TestDeleteOrderIsExactReverseAndFKSafe`.
- `TestAllowlistMatchesLiveSchema` -- introspect a real migrated SQLite temp DB via
  `PRAGMA table_info` per table; fail if the allowlist ever drifts. This is the guard that a
  future migration cannot silently drop a table or column from every backup.
- `TestRegistryExcludesBalanceView`.

`internal/backup/codec_test.go` (no live DB -- this is R7's engine-agnostic layer)
- `TestInt64RoundTripsAsJSONString` -- `math.MaxInt64` and a negative amount marshal as a quoted
  string, never a bare number, and decode back exactly.
- `TestDecodeRejectsBareNumberForInt64Column`.
- `TestBoolColumnEncodingBothDialects` -- `{sqlite -> 0/1}` and `{postgres -> native bool}` for
  the same logical value.
- `TestNullVsEmptyStringPreserved`.
- `TestJSONTextPreservedByteForByte` -- unusual key order and interior whitespace survive.

`internal/backup/crypto_test.go`
- `TestSealOpenRoundTrip`, including a multi-chunk payload; a flipped ciphertext or AAD byte
  fails.
- `TestFingerprintMismatchDetectsWrongSecret` -- surfaces `ErrWrongSessionSecret`, not a raw AEAD
  error.
- `TestSubkeysAreDomainSeparated`.
- `TestTruncatedStreamFails`, `TestReorderedChunksFail`.
- `TestPlaintextHeaderCarriesNoSecret`.

`internal/backup/archive_test.go`
- `TestArchiveHeaderReadableWithoutKey`.
- `TestRejectsPathTraversalAndSymlinksInUploads` -- table-driven over
  `uploads/../../etc/passwd`, an absolute path, a `uploads/C:\Windows\evil` shape, and a
  `TypeSymlink`; no partial staging directory survives any of them.
- `TestRejectsUnknownTopLevelEntries`, `TestRejectsMissingRequiredTable`,
  `TestRejectsDuplicateTableEntry`.
- `TestEnforcesPerEntryAndTotalSizeCaps` -- from actual bytes read, not declared sizes.
- `TestEnforcesJSONLLineLengthCap` -- a named error, not `bufio.ErrTooLong`.

`internal/backup/dump_test.go` (real SQLite)
- `TestDumpArchivedExpensesRoundTrips` -- seeds the two write-only tables via direct SQL, since
  no Go insert path exists, and asserts both appear.
- `TestDumpExcludesBalanceView`.
- `TestDumpSnapshotIsolatesConcurrentSplitEdit` -- mutate `expense_participants` for an
  in-flight expense on a second connection mid-dump; the archived rows for that expense must
  still sum to zero and match either the pre- or post-edit state exactly, never a mix.

`internal/backup/restore_test.go` (real SQLite)
- `TestRestoreRoundTripPreservesIDsAndZeroSumParticipants` -- the headline test. Seed a
  realistic dataset, dump, wipe, restore; every table's rows and PKs unchanged and
  `balance_view` yields identical balances.
- `TestRestoreRejectsSchemaVersionMismatch` -- row counts unchanged after a refused attempt.
- `TestRestoreAcceptsMatchingVersionsWithDifferentAppliedAt` -- the regression guard for the
  `applied_at` exclusion.
- `TestRestoreWrongSessionSecretFailsWithFingerprintError`.
- `TestRestoreRollsBackOnMidTransactionError` -- row counts and `UploadDir` both untouched.
- `TestSequenceResetStatementsOnlyForPostgres` -- asserts the exact SQL produced, no live PG.
- `TestApplyRestoreRemovesOldUploadsDirOnSuccess`.
- `TestRestoreWithoutForceRefusesAndRunsNoDelete`.

`internal/backup/recovery_test.go`
- `TestRecoverIncompleteSwapRevertsMidSwapCrash`,
  `TestRecoverIncompleteSwapCleansOrphanedStaging`.

`internal/backup/autorestore_test.go`
- `TestAutoRestoreSkipsWhenMarkerPresent`, `TestAutoRestoreDisabledWhenDirUnset`.
- `TestAutoRestoreSchemaMismatchAbortsBoot` -- per R9, and asserts no marker is written.
- `TestAutoRestoreAbortsBootWhenMarkerWriteFails`.
- `TestAutoRestoreAbortsOnMultipleArchives`.
- `TestAutoRestoreLockRenewalOutlastsSlowRestore` -- a slow `ApplyRestore` exceeding the
  original TTL still holds the lock; a second concurrent `CheckAutoRestore` does not acquire it.
- `TestAutoRestoreFatalMessageNamesRemediation`.

`cmd/gosplit/backup_cli_test.go` and `main_test.go`
- `TestWantsBackupRoutesSubcommands` -- table-driven like the existing cases, plus `backup`,
  `restore`, `inspect`. Existing `TestWantsVersion` and `TestVersionDefault` unchanged.
- `TestRunBackupCLIUsesOpenNoMigrate` -- a sentinel proves `migrate()` never ran against a DB
  seeded at an older schema version.
- `TestRunRestoreCLIRefusesWithoutForce` -- no DB mutation; prints the manifest preview.

`internal/httpapp/handlers_backup_test.go` (harness style)
- `TestBackupRoutesRequireAdmin` -- anon redirects, non-admin 403, for every route.
- `TestBackupDownloadLinkNotBoosted`, `TestBackupDownloadRejectsTraversalFilename`.
- `TestRestoreUploadOversizeRejectedBeforeTempFile` -- a body over `RestoreMaxUploadMB` is
  rejected and no file appears under `os.TempDir()`.
- `TestRestoreConfirmRejectsWrongPhrase` -- no `ApplyRestore` call, row counts unchanged.
- `TestRestoreUploadRequiresCSRF`.
- `TestRestoreJobStatusReauthenticatesOrClearsSession` -- both branches, via the cookiejar.
- `TestMaintenanceModeRejectsWritesDuringRestore` -- a concurrent POST gets 503 with
  `Retry-After`.
- `TestJobErrorsAreSanitized` -- neither a DSN password nor an offending row value appears in
  `JobStatus.Err`.
- `TestHealthzIncludesVersion`.

`internal/scheduler/scheduler_test.go`
- `TestScheduledBackupRunsOnCronDueAndPrunesRetention`.
- `TestScheduledBackupDoesNotBlockCleanupTick`.

`internal/config/config_test.go`
- `TestBackupCronInvalidFailsLoud`, `TestBackupCronWithoutDirFailsLoud`,
  `TestBackupDirEqualAutoRestoreDirFailsLoud`, `TestBackupConfigDefaults`.

`internal/web/` -- `TestFooterShowsVersion`.

## Docs

- `README.md` -- new "Backup and restore" section under Features: the three triggers with exact
  commands, the auto-restore marker protocol, wipe-and-replace/ID-preserving semantics, the
  cross-engine guarantee, the SQLite maintenance-window note for CLI restores, the accepted
  crash-loop behaviour from R9 and its remediation, and a prominent warning that the archive is
  sealed with a key derived from `SESSION_SECRET` -- rotating or losing it orphans every archive.
  Add footer and `/healthz` to the version surfaces listed at README:253-263.
- **Fix `README.md:216-217`**, which claims Postgres data-layer tests run via
  `TEST_POSTGRES_URL`. That variable appears nowhere in any `.go` file and no such harness
  exists. Replace it with what R7 actually delivers -- SQLite round-trip tests plus
  engine-agnostic codec tests -- and say plainly that cross-engine restore is verified by hand.
- `.env.example` -- a new `# --- backup / restore ---` block in the existing comment style,
  documenting every field from step 2 plus the `SESSION_SECRET` coupling warning.
- `docs/backup-restore-plan.md` -- this plan in the house format
  (`docs/ux-responsiveness-plan.md`), Ledger updated as steps land.
- `docker-compose.yml` -- commented-out `BACKUP_DIR` / `AUTO_RESTORE_DIR` examples, matching how
  the Postgres service is presented there today.
- `scripts/dev.sh` / `dev.ps1` -- **no new task.** `build`, `vet`, `test`, `cov` and `scan`
  already cover `internal/backup` and `cmd/gosplit` with zero wiring. Called out explicitly
  rather than silently skipped.

## Explicitly deferred / rejected

- **A `BACKUP_KEY` override or a `rekey` subcommand.** Decided against (R5). The consequence is
  documented rather than mitigated.
- **A live Postgres test harness.** Decided against (R7). Postgres-only logic (sequence resets,
  bool binding, `LOCK TABLE`) is covered by unit tests asserting the exact SQL and values
  produced, never against a live engine.
- **`pg_dump` / `sqlite3 .backup` / `VACUUM INTO`.** Impossible in the distroless runtime -- no
  shell, no external binaries -- and none of them is cross-engine.
- **`SET CONSTRAINTS ALL DEFERRED`.** Ineffective against `NOT DEFERRABLE` constraints and has
  no SQLite equivalent. Strict topological ordering is used instead.
- **A `serve` subcommand.** `main_test.go` anticipates one, but nothing here needs it: bare
  invocation keeps starting the server exactly as today. Adding it is an independent change.
- **Incremental or differential backups.** Whole-instance archives only; the retention count is
  the size control.
- **Seamless multi-replica auto-restore coordination** beyond the renewed leader lock. A losing
  replica boots without restoring; this is a safety net, not a distributed restore protocol. The
  README tells multi-replica Postgres deployments to prefer the CLI trigger, or to point
  `AUTO_RESTORE_DIR` at one replica only.
- **A localized restore-confirmation phrase.** Kept as the fixed literal `RESTORE ALL DATA`.
- **Restoring `balance_view`.** Derived; restoring it would create a second source of truth for
  money.

## Verification (user-run)

Gates, once the code lands -- the same invocation CI runs:

```
./scripts/dev.sh all scan
```

or on Windows:

```
./scripts/dev.ps1 all scan
```

Compare the total `cov` prints against the previous cov log. Then `graphify update .` per project
convention.

Manual end-to-end checks:

1. **Version surfaces.** `./scripts/dev.ps1 build`, then
   `dist/gosplit-windows-amd64.exe --version`. Start the app and confirm the tag appears in the
   footer and in `curl http://localhost:8080/healthz`.
2. **SQLite round-trip.** Create groups, expenses, a settlement and a collapsed ("archived")
   batch, and upload an avatar. `docker exec <ctr> /app/gosplit backup -o /data/backups`. Note
   the row counts. Mutate the data further, then
   `docker exec <ctr> /app/gosplit restore --file /data/backups/<file>.gsbak --force`. Confirm
   the mutation is gone, every balance matches, the collapsed history is intact, and the avatar
   renders.
3. **Cross-engine restore.** Uncomment the `db` service in `docker-compose.yml`, point
   `DATABASE_URL` at Postgres, and restore the SQLite-made archive into it. Confirm balances
   match, that `\d groups` shows `simplify_debts` as `boolean` with correct values, and that
   creating a new group afterwards gets an id past the restored max -- that last check is what
   proves the sequence reset worked. Then back up from Postgres and restore into a fresh SQLite
   file.
4. **Wrong secret.** Restore an archive with a different `SESSION_SECRET`; the error must name
   the fingerprint mismatch and say what to do.
5. **Preview is read-only.** Run `restore --file <path>` without `--force`: it prints the
   manifest summary and changes nothing.
6. **Admin panel.** As an admin, generate and download from `/admin/backup` -- confirm the
   browser saves a `.gsbak` file rather than swapping in a page (the `hx-boost="false"` check).
   Upload it back, confirm the preview shows correct table counts, type `RESTORE ALL DATA`, and
   confirm you end up on an authenticated admin session if your own user survived, or a clean
   "log in again" if not. Try a wrong phrase and confirm nothing changed. During the restore,
   confirm a concurrent write in another tab gets a 503 rather than hanging.
7. **Scheduled backup.** Set `BACKUP_CRON="* * * * *"` and `BACKUP_RETENTION_COUNT=2`, wait three
   minutes, and confirm exactly two archives remain. Set an invalid cron rule and confirm the app
   refuses to boot with a clear message. Set `BACKUP_DIR` equal to `AUTO_RESTORE_DIR` and confirm
   it also refuses.
8. **Startup auto-restore.** Set `AUTO_RESTORE_DIR=/data/restore`, drop an archive there, and
   restart. Confirm the restore runs, the marker appears, a safety dump was written, and the
   archive is still in place. Restart again and confirm a no-op. Delete the marker and confirm it
   restores again. Then corrupt the archive and restart: the container must exit non-zero, leave
   the database untouched, and log a message naming the remediation. Remove the file, restart,
   and confirm normal boot.
9. **Hostile archive.** Hand-craft archives containing (a) an `uploads/../evil` entry and (b) a
   missing `tables/expense_participants.jsonl`, and confirm both are refused before the database
   is touched.
