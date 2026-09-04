package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// Runner carries the dependencies a dump or restore needs.
type Runner struct {
	Store *store.Store
	Cfg   *config.Config
	// Now is injected so archive names and timestamps are deterministic under
	// test.
	Now func() time.Time
}

// New builds a Runner.
func New(st *store.Store, cfg *config.Config) *Runner {
	return &Runner{Store: st, Cfg: cfg, Now: func() time.Time { return time.Now().UTC() }}
}

func (r *Runner) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

// isoFormat matches the timestamp format the rest of the app stores
// (store.nowISO), so an archive's created_at reads like every other timestamp.
const isoFormat = "2006-01-02T15:04:05.000Z"

// queryer is the read surface a dump needs. Both *sql.Tx and *sql.Conn satisfy
// it, which is what lets the two engines use different snapshot mechanisms
// behind one code path.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Dump writes a sealed archive of the whole instance to w.
//
// Every table is read inside ONE consistent snapshot. That is not an
// optimization: store.UpdateExpense implements a split edit as a DELETE of an
// expense's participant rows followed by re-INSERTs, so a dump that read
// expenses and expense_participants in separate transactions could capture the
// delete without the re-insert. The archive's participant rows would then no
// longer sum to zero, and restoring it would silently misstate every balance.
func (r *Runner) Dump(ctx context.Context, w io.Writer) (Manifest, error) {
	q, release, err := r.snapshot(ctx)
	if err != nil {
		return Manifest{}, err
	}
	defer release()

	migrations, err := readMigrationVersions(ctx, q)
	if err != nil {
		return Manifest{}, err
	}

	salt, err := NewSalt()
	if err != nil {
		return Manifest{}, err
	}
	prefix, err := NewNoncePrefix()
	if err != nil {
		return Manifest{}, err
	}
	k, err := NewKeyring(r.Cfg.SessionSecret, salt)
	if err != nil {
		return Manifest{}, err
	}

	created := r.now().Format(isoFormat)
	hdr := Header{
		FormatVersion:        FormatVersion,
		CreatedAt:            created,
		SourceEngine:         string(r.Store.Engine),
		GosplitVersion:       r.Cfg.AppVersion,
		KDFAlgo:              "scrypt",
		KDFN:                 scryptN,
		KDFR:                 scryptR,
		KDFP:                 scryptP,
		SaltB64:              b64(salt),
		KeyFingerprintB64:    b64(k.Fingerprint()),
		AEADAlgo:             "AES-256-GCM",
		NoncePrefixB64:       b64(prefix),
		AEADChunkSizeHint:    ChunkSize,
		SchemaMigrationsHint: migrations,
	}
	aad, err := WriteHeader(w, hdr)
	if err != nil {
		return Manifest{}, err
	}

	sealed, err := NewSealWriter(w, k, prefix, aad)
	if err != nil {
		return Manifest{}, err
	}
	gz := gzip.NewWriter(sealed)
	tw := tar.NewWriter(gz)

	man := Manifest{
		FormatVersion:    FormatVersion,
		CreatedAt:        created,
		SourceEngine:     string(r.Store.Engine),
		GosplitVersion:   r.Cfg.AppVersion,
		SchemaMigrations: migrations,
	}

	// Tables first, then uploads, then the manifest LAST: writing it last means
	// its row and upload counts are what was actually written, not what a
	// pre-pass predicted. Scan does not care about entry order.
	for _, t := range InsertOrder() {
		n, err := r.dumpTable(ctx, q, tw, t)
		if err != nil {
			return Manifest{}, fmt.Errorf("backup: dump table %s: %w", t.Name, err)
		}
		man.Tables = append(man.Tables, TableStat{Name: t.Name, RowCount: n})
	}

	files, bytesWritten, err := r.dumpUploads(tw)
	if err != nil {
		return Manifest{}, err
	}
	man.UploadFileCount = files
	man.UploadTotalBytes = bytesWritten

	raw, err := json.Marshal(man)
	if err != nil {
		return Manifest{}, fmt.Errorf("backup: encode manifest: %w", err)
	}
	if err := writeTarFile(tw, ManifestEntry, int64(len(raw)), bytes.NewReader(raw)); err != nil {
		return Manifest{}, err
	}

	if err := tw.Close(); err != nil {
		return Manifest{}, fmt.Errorf("backup: finish tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return Manifest{}, fmt.Errorf("backup: finish gzip: %w", err)
	}
	// Closing the seal writer emits the final-flagged frame. Without it a
	// reader correctly reports the archive as truncated.
	if err := sealed.Close(); err != nil {
		return Manifest{}, fmt.Errorf("backup: finish sealed payload: %w", err)
	}
	return man, nil
}

// DumpToDir writes an archive into dir under a generated name, via a .partial
// file that is renamed on success -- so nothing scanning the directory (the
// startup auto-restore especially) can ever pick up a half-written archive.
func (r *Runner) DumpToDir(ctx context.Context, dir, prefix string) (string, Manifest, error) {
	if dir == "" {
		return "", Manifest{}, fmt.Errorf("backup: no output directory configured (set BACKUP_DIR or pass -o)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", Manifest{}, fmt.Errorf("backup: create %s: %w", dir, err)
	}
	name := fmt.Sprintf("%s-%s%s", prefix, r.now().Format("20060102T150405Z"), ArchiveExt)
	final := filepath.Join(dir, name)
	partial := final + ".partial"

	// 0600: the archive holds password hashes, session tokens and bank data.
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", Manifest{}, fmt.Errorf("backup: create %s: %w", partial, err)
	}
	man, dumpErr := r.Dump(ctx, f)
	closeErr := f.Close()
	if dumpErr != nil {
		_ = os.Remove(partial)
		return "", Manifest{}, dumpErr
	}
	if closeErr != nil {
		_ = os.Remove(partial)
		return "", Manifest{}, fmt.Errorf("backup: close %s: %w", partial, closeErr)
	}
	if err := os.Rename(partial, final); err != nil {
		_ = os.Remove(partial)
		return "", Manifest{}, fmt.Errorf("backup: rename %s to %s: %w", partial, final, err)
	}
	return final, man, nil
}

// snapshot opens a consistent read view, per engine, and returns a release
// function.
func (r *Runner) snapshot(ctx context.Context) (queryer, func(), error) {
	if r.Store.Engine == config.EnginePostgres {
		// Postgres pools freely, so the normal handle is fine and REPEATABLE
		// READ gives a true snapshot for the transaction's duration.
		tx, err := r.Store.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
		if err != nil {
			return nil, nil, fmt.Errorf("backup: begin snapshot transaction: %w", err)
		}
		return tx, func() { _ = tx.Rollback() }, nil
	}

	// SQLite: take a SEPARATE read-only handle rather than the server's pool,
	// which is pinned to a single connection (store.connect). A long read
	// transaction on that one connection would stall every HTTP request for
	// the whole dump; WAL lets an independent reader run without blocking the
	// writer.
	dsn := store.SQLiteReadOnlyDSN(r.Cfg)
	if dsn == "" {
		// An in-memory database has no second handle, so fall back to the
		// pooled one. Only tests use this, and they have no concurrent writer.
		tx, err := r.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, nil, fmt.Errorf("backup: begin snapshot transaction: %w", err)
		}
		return tx, func() { _ = tx.Rollback() }, nil
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("backup: open read-only snapshot handle: %w", err)
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("backup: acquire snapshot connection: %w", err)
	}
	// BEGIN DEFERRED on a pinned connection: in WAL mode the read transaction
	// sees one consistent snapshot from its first read until it ends.
	if _, err := conn.ExecContext(ctx, "BEGIN DEFERRED"); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, nil, fmt.Errorf("backup: begin snapshot transaction: %w", err)
	}
	release := func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "COMMIT")
		_ = conn.Close()
		_ = db.Close()
	}
	return conn, release, nil
}

// dumpTable streams one table into bounded parts. Rows are ordered by the
// table's own columns so that two dumps of identical data produce identical
// bytes, which lets a round-trip test assert equality rather than set
// membership.
func (r *Runner) dumpTable(ctx context.Context, q queryer, tw *tar.Writer, t Table) (int64, error) {
	cols := t.ColumnNames()
	// Ordered by the primary key: deterministic bytes for identical data, and
	// it uses the existing index rather than sorting on every column.
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s",
		quoteIdents(cols), quoteIdent(t.Name), quoteIdents(t.PKColumns()))
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	dest := make([]any, len(t.Columns))
	var (
		buf   bytes.Buffer
		part  int
		total int64
	)
	for rows.Next() {
		for i, c := range t.Columns {
			dest[i] = ScanDest(c.Kind)
		}
		if err := rows.Scan(dest...); err != nil {
			return 0, err
		}
		row := make([]json.RawMessage, len(t.Columns))
		for i, c := range t.Columns {
			v, err := EncodeColumn(c.Kind, dest[i])
			if err != nil {
				return 0, fmt.Errorf("column %s: %w", c.Name, err)
			}
			row[i] = v
		}
		line, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		if buf.Len() > 0 && buf.Len()+len(line)+1 > tablePartMaxBytes {
			if err := writeTarFile(tw, tablePartName(t.Name, part), int64(buf.Len()), bytes.NewReader(buf.Bytes())); err != nil {
				return 0, err
			}
			part++
			buf.Reset()
		}
		buf.Write(line)
		buf.WriteByte('\n')
		total++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	// Always emit part 0, even for an empty table: a missing part is how a
	// restore detects an archive that would silently empty the table, so an
	// empty table must be represented by an empty part rather than by absence.
	if err := writeTarFile(tw, tablePartName(t.Name, part), int64(buf.Len()), bytes.NewReader(buf.Bytes())); err != nil {
		return 0, err
	}
	return total, nil
}

// dumpUploads copies the upload tree into the archive. Uploads are not covered
// by the database snapshot; that window is accepted because an upload file is
// immutable once written.
func (r *Runner) dumpUploads(tw *tar.Writer) (int, int64, error) {
	dir := r.Cfg.UploadDir
	if dir == "" {
		return 0, 0, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, 0, nil
	}
	var (
		files int
		total int64
	)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Regular files only: a symlink or device in the upload directory is
		// not something to carry into an archive.
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		slashRel := filepath.ToSlash(rel)
		if _, err := safeUploadRel(uploadsPrefix + slashRel); err != nil {
			// A single unusable name is skipped rather than aborting the whole
			// backup, but it is not skipped silently.
			return fmt.Errorf("backup: upload %q cannot be archived: %w", slashRel, err)
		}
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				// Removed between the walk and the open; nothing to archive.
				return nil
			}
			return err
		}
		defer func() { _ = f.Close() }()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		size := st.Size()
		if err := writeTarFile(tw, uploadsPrefix+slashRel, size, io.LimitReader(f, size)); err != nil {
			return err
		}
		files++
		total += size
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("backup: archive uploads from %s: %w", dir, err)
	}
	return files, total, nil
}

// writeTarFile writes one regular-file entry of exactly size bytes. A short
// read means the source changed mid-dump, which would corrupt the tar, so it
// is reported rather than padded over.
func writeTarFile(tw *tar.Writer, name string, size int64, src io.Reader) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     size,
		Typeflag: tar.TypeReg,
		// A fixed modtime keeps two dumps of identical data byte-identical.
		ModTime: time.Unix(0, 0).UTC(),
		Format:  tar.FormatPAX,
	}); err != nil {
		return fmt.Errorf("backup: write tar header for %s: %w", name, err)
	}
	n, err := io.Copy(tw, src)
	if err != nil {
		return fmt.Errorf("backup: write %s: %w", name, err)
	}
	if n != size {
		return fmt.Errorf("backup: %s changed while it was being archived (expected %d bytes, wrote %d)", name, size, n)
	}
	return nil
}

// readMigrationVersions reads the applied migration filenames, sorted.
// applied_at is deliberately not read: see Manifest.SchemaMigrations.
func readMigrationVersions(ctx context.Context, q queryer) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+quoteIdent("version")+` FROM `+quoteIdent("schema_migrations"))
	if err != nil {
		return nil, fmt.Errorf("backup: read schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// SameMigrationSet reports whether two sorted migration sets are identical,
// and names the difference when they are not.
func SameMigrationSet(archive, live []string) (bool, string) {
	a := append([]string(nil), archive...)
	l := append([]string(nil), live...)
	sort.Strings(a)
	sort.Strings(l)
	if strings.Join(a, ",") == strings.Join(l, ",") {
		return true, ""
	}
	inLive := map[string]bool{}
	for _, v := range l {
		inLive[v] = true
	}
	inArchive := map[string]bool{}
	for _, v := range a {
		inArchive[v] = true
	}
	var onlyArchive, onlyLive []string
	for _, v := range a {
		if !inLive[v] {
			onlyArchive = append(onlyArchive, v)
		}
	}
	for _, v := range l {
		if !inArchive[v] {
			onlyLive = append(onlyLive, v)
		}
	}
	var parts []string
	if len(onlyArchive) > 0 {
		parts = append(parts, "only in the archive: "+strings.Join(onlyArchive, ", "))
	}
	if len(onlyLive) > 0 {
		parts = append(parts, "only in this database: "+strings.Join(onlyLive, ", "))
	}
	return false, strings.Join(parts, "; ")
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
