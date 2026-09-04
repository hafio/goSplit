package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hafio/gosplit/internal/config"
)

// A restore is irreversible: it empties every table and replaces the upload
// tree. So the work is split in two. Validate does every check and stages the
// uploads while touching nothing live, and Apply is the short destructive
// window. Anything that can be refused is refused in Validate.

// ErrSchemaMismatch means the archive was written against a different set of
// migrations than the target database has applied.
var ErrSchemaMismatch = errors.New("backup: archive schema does not match this database")

// ErrRestoreNotConfirmed means a destructive restore was requested without the
// explicit confirmation each trigger requires.
var ErrRestoreNotConfirmed = errors.New("backup: restore not confirmed")

// maxBindParams bounds one multi-row INSERT. SQLite (as compiled into
// modernc.org/sqlite) allows 32766 bound parameters and Postgres 65535; 16000
// is comfortably inside both.
const maxBindParams = 16000

// maxBatchRows bounds a batch independently of column count, to keep generated
// statements a reasonable size.
const maxBatchRows = 1000

// Validated is an archive that has passed every check, with its uploads staged
// and ready to swap in. It is produced once and handed to Apply unchanged, so
// what was validated is exactly what gets applied.
type Validated struct {
	ArchivePath string
	Header      Header
	Manifest    Manifest
	Index       *Index

	opener  Opener
	keyring *Keyring
	aad     []byte
	limits  Limits
	staging string
}

// ApplyOptions controls the destructive step.
type ApplyOptions struct {
	// Confirmed must be true. Each trigger sets it only after its own
	// ceremony: --force on the CLI, a typed phrase in the admin panel, the
	// marker-file protocol at startup.
	Confirmed bool
	// SkipSafetyDump disables the pre-restore dump of current data. It exists
	// for tests and for an operator who has just taken a backup by hand;
	// leaving it false is strongly preferred.
	SkipSafetyDump bool
	// SafetyDumpDir overrides where the pre-restore dump is written.
	SafetyDumpDir string
}

// Report describes what a restore did.
type Report struct {
	Manifest        Manifest
	SafetyDumpPath  string
	RowsRestored    map[string]int64
	UploadsRestored int
}

// InspectFile reads an archive's plaintext header. It needs no key, no
// database and no configuration, and touches nothing, so it is safe to point
// at any file -- including one sealed with a secret this instance does not
// have.
func InspectFile(path string) (Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return Header{}, fmt.Errorf("backup: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	hdr, _, _, err := ReadHeader(f)
	return hdr, err
}

// Validate runs every check that can be made without mutating anything, and
// stages the archive's uploads beside the live upload directory.
//
// It never writes to the database or to the live upload directory, so a
// failure costs nothing but the staging directory.
func (r *Runner) Validate(ctx context.Context, archivePath string) (*Validated, error) {
	opener := FileOpener(archivePath)

	rc, err := opener()
	if err != nil {
		return nil, fmt.Errorf("backup: open %s: %w", archivePath, err)
	}
	hdr, aad, _, err := ReadHeader(rc)
	_ = rc.Close()
	if err != nil {
		return nil, err
	}

	salt, err := hdr.Salt()
	if err != nil {
		return nil, err
	}
	k, err := NewKeyring(r.Cfg.SessionSecret, salt)
	if err != nil {
		return nil, err
	}
	recorded, err := hdr.KeyFingerprint()
	if err != nil {
		return nil, err
	}
	// Checked BEFORE any decryption, so a rotated or mistyped secret produces
	// a message that says so rather than an opaque authentication failure.
	if !k.FingerprintMatches(recorded) {
		return nil, ErrWrongSessionSecret
	}

	staging, err := r.prepareStaging()
	if err != nil {
		return nil, err
	}

	limits := LimitsFrom(r.Cfg)
	idx, err := Scan(opener, k, hdr, aad, limits, staging)
	if err != nil {
		_ = os.RemoveAll(staging)
		return nil, err
	}

	live, err := r.liveMigrationVersions(ctx)
	if err != nil {
		_ = os.RemoveAll(staging)
		return nil, err
	}
	// Compared on version strings only. applied_at is deliberately excluded:
	// it records when a particular database first saw a migration, not the
	// migration's identity, so including it would make every restore onto a
	// freshly provisioned database -- the primary disaster-recovery case --
	// fail as a spurious mismatch.
	//
	// The match must be exact. That is what stops a data migration such as
	// 0005_simplify_debts_default.sql -- a blanket
	// "UPDATE groups SET simplify_debts = 1" -- from re-firing against
	// restored rows on a later boot and clobbering every group's flag.
	if same, diff := SameMigrationSet(idx.Manifest.SchemaMigrations, live); !same {
		_ = os.RemoveAll(staging)
		return nil, fmt.Errorf("%w (%s); restore this archive with the build whose migrations match it", ErrSchemaMismatch, diff)
	}

	return &Validated{
		ArchivePath: archivePath,
		Header:      hdr,
		Manifest:    idx.Manifest,
		Index:       idx,
		opener:      opener,
		keyring:     k,
		aad:         aad,
		limits:      limits,
		staging:     staging,
	}, nil
}

// Discard releases a validated archive's staging directory without applying
// it. Safe to call more than once.
func (v *Validated) Discard() {
	if v == nil || v.staging == "" {
		return
	}
	_ = os.RemoveAll(v.staging)
	v.staging = ""
}

// Apply performs the destructive restore: one transaction empties every table
// and reinserts the archive's rows with their original primary keys, then the
// staged uploads are swapped in.
func (r *Runner) Apply(ctx context.Context, v *Validated, opts ApplyOptions) (Report, error) {
	if v == nil {
		return Report{}, errors.New("backup: no validated archive to apply")
	}
	if !opts.Confirmed {
		return Report{}, ErrRestoreNotConfirmed
	}

	rep := Report{Manifest: v.Manifest, RowsRestored: map[string]int64{}}

	// The operator's only way back. If it fails, nothing is touched.
	if !opts.SkipSafetyDump {
		dir := opts.SafetyDumpDir
		if dir == "" {
			dir = r.Cfg.BackupDir
		}
		path, _, err := r.DumpToDir(ctx, dir, SafetyDumpPrefix)
		if err != nil {
			return Report{}, fmt.Errorf("backup: pre-restore safety dump failed, so the restore was not attempted: %w", err)
		}
		rep.SafetyDumpPath = path
	}

	if err := r.applyTx(ctx, v, &rep); err != nil {
		return Report{}, err
	}

	// The upload tree cannot join a SQL transaction, so it is swapped after the
	// commit, with two same-filesystem renames.
	n, err := r.swapUploads(v)
	if err != nil {
		return Report{}, err
	}
	rep.UploadsRestored = n
	v.staging = ""
	return rep, nil
}

// applyTx is the single transaction. Any error rolls the whole thing back, so
// a failed restore leaves the database exactly as it was.
func (r *Runner) applyTx(ctx context.Context, v *Validated, rep *Report) error {
	tx, err := r.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("backup: begin restore transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if r.Store.Engine == config.EnginePostgres {
		// Block every concurrent writer at the engine level for the duration.
		// This covers a restore triggered from another process (the CLI) as
		// well as this one, which the in-process maintenance flag cannot.
		for _, t := range InsertOrder() {
			if _, err := tx.ExecContext(ctx, "LOCK TABLE "+quoteIdent(t.Name)+" IN ACCESS EXCLUSIVE MODE"); err != nil {
				return fmt.Errorf("backup: lock table %s: %w", t.Name, err)
			}
		}
	}

	// Empty every table, children first.
	for _, t := range DeleteOrder() {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoteIdent(t.Name)); err != nil {
			return fmt.Errorf("backup: clear table %s: %w", t.Name, err)
		}
	}

	// Reinsert, parents first. ForEachRow walks the archive in InsertOrder, so
	// batches only ever need flushing when the table changes or fills up.
	b := &batcher{tx: tx, engine: r.Store.Engine, rebind: r.Store.Rebind, counts: rep.RowsRestored}
	if err := ForEachRow(v.opener, v.keyring, v.Header, v.aad, v.limits, v.Index, func(t Table, row []json.RawMessage) error {
		return b.add(ctx, t, row)
	}); err != nil {
		return err
	}
	if err := b.flush(ctx); err != nil {
		return err
	}

	// Postgres identity sequences still point at the pre-restore high-water
	// mark, so without this the next insert collides with a restored id.
	if r.Store.Engine == config.EnginePostgres {
		for _, t := range InsertOrder() {
			if t.Sequence == "" {
				continue
			}
			stmt := fmt.Sprintf(
				"SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE((SELECT MAX(%s) FROM %s), 0) + 1, false)",
				t.Name, quoteIdent("id"), quoteIdent(t.Name))
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("backup: reset sequence for %s: %w", t.Name, err)
			}
		}
	}

	// Row counts are verified, not assumed. ForEachRow already checked the
	// archive against its manifest; this checks what actually landed.
	for _, t := range InsertOrder() {
		want, _ := v.Manifest.RowCount(t.Name)
		var got int64
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdent(t.Name)).Scan(&got); err != nil {
			return fmt.Errorf("backup: count rows in %s: %w", t.Name, err)
		}
		if got != want {
			return fmt.Errorf("backup: table %s holds %d rows after the restore but the archive records %d; rolling back", t.Name, got, want)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("backup: commit restore: %w", err)
	}
	committed = true
	return nil
}

// batcher turns a stream of rows into multi-row INSERTs. Statement text comes
// only from the registry; every archive value is a bound parameter.
type batcher struct {
	tx     *sql.Tx
	engine config.Engine
	rebind func(string) string
	table  Table
	args   []any
	rows   int
	counts map[string]int64
}

func (b *batcher) add(ctx context.Context, t Table, row []json.RawMessage) error {
	if b.rows > 0 && b.table.Name != t.Name {
		if err := b.flush(ctx); err != nil {
			return err
		}
	}
	b.table = t
	for i, c := range t.Columns {
		arg, err := DecodeColumn(b.engine, c.Kind, row[i])
		if err != nil {
			return fmt.Errorf("table %s column %s: %w", t.Name, c.Name, err)
		}
		b.args = append(b.args, arg)
	}
	b.rows++
	if b.rows >= maxBatchRows || (b.rows+1)*len(t.Columns) > maxBindParams {
		return b.flush(ctx)
	}
	return nil
}

func (b *batcher) flush(ctx context.Context) error {
	if b.rows == 0 {
		return nil
	}
	cols := b.table.ColumnNames()
	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(quoteIdent(b.table.Name))
	sb.WriteString(" (")
	sb.WriteString(quoteIdents(cols))
	sb.WriteString(") VALUES ")
	for row := 0; row < b.rows; row++ {
		if row > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('(')
		for i := range cols {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteByte('?')
		}
		sb.WriteByte(')')
	}
	if _, err := b.tx.ExecContext(ctx, b.rebind(sb.String()), b.args...); err != nil {
		return fmt.Errorf("backup: insert into %s: %w", b.table.Name, err)
	}
	b.counts[b.table.Name] += int64(b.rows)
	b.args = b.args[:0]
	b.rows = 0
	return nil
}

// prepareStaging makes a fresh staging directory as a SIBLING of the upload
// directory, so the swap is a same-filesystem rename. A leftover from an
// interrupted attempt is cleared first.
func (r *Runner) prepareStaging() (string, error) {
	if r.Cfg.UploadDir == "" {
		return "", errors.New("backup: UPLOAD_DIR is not configured")
	}
	staging := stagingDirFor(r.Cfg.UploadDir)
	if err := os.RemoveAll(staging); err != nil {
		return "", fmt.Errorf("backup: clear stale staging directory %s: %w", staging, err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", fmt.Errorf("backup: create staging directory %s: %w", staging, err)
	}
	return staging, nil
}

// swapUploads replaces the live upload tree with the staged one using two
// renames, rolling the first back if the second fails.
func (r *Runner) swapUploads(v *Validated) (int, error) {
	live := r.Cfg.UploadDir
	old := oldDirFor(live)
	staging := v.staging

	if err := os.RemoveAll(old); err != nil {
		return 0, fmt.Errorf("backup: clear stale %s: %w", old, err)
	}
	moved := false
	if _, err := os.Stat(live); err == nil {
		if err := os.Rename(live, old); err != nil {
			return 0, fmt.Errorf("backup: move the live upload directory aside: %w", err)
		}
		moved = true
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("backup: inspect %s: %w", live, err)
	}
	if err := os.Rename(staging, live); err != nil {
		if moved {
			// Put the original back rather than leaving no upload directory.
			_ = os.Rename(old, live)
		}
		return 0, fmt.Errorf("backup: move the restored uploads into place: %w", err)
	}
	// The pre-restore safety dump already contains the previous uploads, so
	// keeping this copy too would grow without bound across restores.
	if err := os.RemoveAll(old); err != nil {
		return len(v.Index.Uploads), fmt.Errorf("backup: uploads restored, but the previous directory %s could not be removed: %w", old, err)
	}
	return len(v.Index.Uploads), nil
}

// stagingDirFor and oldDirFor use fixed suffixes rather than timestamps so
// RecoverIncompleteSwap can recognize an interrupted swap at boot.
func stagingDirFor(uploadDir string) string { return strings.TrimRight(uploadDir, `/\`) + ".restore" }
func oldDirFor(uploadDir string) string     { return strings.TrimRight(uploadDir, `/\`) + ".old" }

// liveMigrationVersions reads the target database's applied migration set.
func (r *Runner) liveMigrationVersions(ctx context.Context) ([]string, error) {
	return readMigrationVersions(ctx, r.Store.DB)
}

// SafetyDump writes a dump of the current data into the configured backup
// directory.
func (r *Runner) SafetyDump(ctx context.Context) (string, error) {
	path, _, err := r.DumpToDir(ctx, r.Cfg.BackupDir, SafetyDumpPrefix)
	return path, err
}
