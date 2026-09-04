package backup

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hafio/gosplit/internal/config"
)

// This file parses an archive that could have come from anywhere. Everything
// here is a guard, and every guard fails the whole restore rather than skipping
// an entry: a malformed archive is a systemic problem, not one bad row.
//
// Sizes are measured from bytes actually read, never from the size a tar or
// gzip header claims. That is what makes the decompression-bomb limit real --
// a claimed size is attacker-controlled and costs nothing to lie about.

// Limits bounds what a restore will consume.
type Limits struct {
	MaxTotalBytes      int64
	MaxUploadFileBytes int64
	MaxTablePartBytes  int64
	MaxEntries         int
	MaxJSONLLineBytes  int
}

// LimitsFrom builds the limits from configuration.
func LimitsFrom(cfg *config.Config) Limits {
	return Limits{
		MaxTotalBytes:      cfg.RestoreMaxArchiveBytes,
		MaxUploadFileBytes: int64(cfg.UploadMaxFileSizeMB) << 20,
		MaxTablePartBytes:  cfg.RestoreMaxTableFileBytes,
		MaxEntries:         cfg.RestoreMaxEntries,
		MaxJSONLLineBytes:  cfg.RestoreMaxJSONLLineBytes,
	}
}

// EntryKind distinguishes the three things a payload may hold.
type EntryKind int

const (
	entryManifest EntryKind = iota
	entryTablePart
	entryUpload
)

// Entry is one validated payload entry.
type Entry struct {
	Name string // the exact tar entry name
	Kind EntryKind
	// Table and Part are set for a table part.
	Table string
	Part  int
	// RelPath is the cleaned slash-separated path under UploadDir, for uploads.
	RelPath string
	// Size is the bytes actually read for this entry.
	Size int64
}

// Index is the single canonical view of a payload, built once by Scan and
// reused by everything downstream. Nothing re-derives its own view of the
// archive, so what was validated is exactly what gets applied.
type Index struct {
	Manifest Manifest
	// TableParts maps a table name to its parts, ordered by part number and
	// verified contiguous from zero.
	TableParts map[string][]Entry
	Uploads    []Entry
	TotalBytes int64
}

// Opener reopens the archive from the beginning. Scan and ForEachRow each make
// one forward pass, so the payload is decrypted twice rather than staged to
// disk as plaintext: the table data holds password hashes, session tokens and
// bank data, and none of it should touch the filesystem unsealed.
type Opener func() (io.ReadCloser, error)

// FileOpener returns an Opener for a path on disk.
func FileOpener(path string) Opener {
	return func() (io.ReadCloser, error) { return os.Open(path) }
}

// Scan makes the validating pass over an archive: it checks every structural
// rule, captures the manifest, and extracts the upload tree into
// uploadStagingDir. It never touches the live database or the live upload
// directory, so any failure here costs nothing but the staging directory.
//
// Pass "" for uploadStagingDir to validate without extracting (inspect, and
// the admin panel's preview).
func Scan(opener Opener, k *Keyring, hdr Header, aad []byte, limits Limits, uploadStagingDir string) (*Index, error) {
	rc, err := opener()
	if err != nil {
		return nil, fmt.Errorf("backup: open archive: %w", err)
	}
	defer func() { _ = rc.Close() }()

	tr, counted, err := openPayload(rc, k, hdr, aad, limits)
	if err != nil {
		return nil, err
	}

	if uploadStagingDir != "" {
		if err := os.MkdirAll(uploadStagingDir, 0o755); err != nil {
			return nil, fmt.Errorf("backup: create upload staging directory %s: %w", uploadStagingDir, err)
		}
	}

	idx := &Index{TableParts: map[string][]Entry{}}
	seen := map[string]bool{}
	var manifestSeen bool
	entries := 0

	for {
		th, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("backup: read archive payload: %w", err)
		}
		entries++
		if entries > limits.MaxEntries {
			return nil, fmt.Errorf("backup: archive holds more than %d entries (RESTORE_MAX_ENTRIES)", limits.MaxEntries)
		}

		// Only regular files and directories. A symlink, hardlink, device or
		// fifo entry has no legitimate place here and is the classic way to
		// write outside the extraction root.
		switch th.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, fmt.Errorf("backup: archive entry %q has disallowed type %q (only regular files and directories are accepted)", th.Name, string(rune(th.Typeflag)))
		}

		name := th.Name
		if seen[name] {
			return nil, fmt.Errorf("backup: archive holds a duplicate entry %q (which of the two applies would be ambiguous)", name)
		}
		seen[name] = true

		switch {
		case name == ManifestEntry:
			raw, err := readCapped(tr, maxHeaderLen)
			if err != nil {
				return nil, fmt.Errorf("backup: read %s: %w", ManifestEntry, err)
			}
			m, err := decodeManifest(raw)
			if err != nil {
				return nil, err
			}
			idx.Manifest = m
			manifestSeen = true

		case strings.HasPrefix(name, tablesPrefix):
			e, err := parseTablePart(name)
			if err != nil {
				return nil, err
			}
			n, err := drainCapped(tr, limits.MaxTablePartBytes)
			if err != nil {
				return nil, fmt.Errorf("backup: entry %q exceeds RESTORE_MAX_TABLE_FILE_MB: %w", name, err)
			}
			e.Size = n
			idx.TableParts[e.Table] = append(idx.TableParts[e.Table], e)

		case strings.HasPrefix(name, uploadsPrefix):
			rel, err := safeUploadRel(name)
			if err != nil {
				return nil, err
			}
			e := Entry{Name: name, Kind: entryUpload, RelPath: rel}
			n, err := extractUpload(tr, uploadStagingDir, rel, limits.MaxUploadFileBytes)
			if err != nil {
				return nil, err
			}
			e.Size = n
			idx.Uploads = append(idx.Uploads, e)

		default:
			return nil, fmt.Errorf("backup: archive holds unexpected entry %q (expected only %s, %s* and %s*)", name, ManifestEntry, tablesPrefix, uploadsPrefix)
		}

		if counted.n > limits.MaxTotalBytes {
			return nil, fmt.Errorf("backup: archive expands past %d bytes (RESTORE_MAX_ARCHIVE_MB); refusing to continue", limits.MaxTotalBytes)
		}
	}

	idx.TotalBytes = counted.n
	if !manifestSeen {
		return nil, fmt.Errorf("backup: archive has no %s", ManifestEntry)
	}
	if err := idx.verifyComplete(); err != nil {
		return nil, err
	}
	return idx, nil
}

// verifyComplete enforces the rules that need the whole payload in view.
//
// The important one is that EVERY registry table must be present. Rejecting
// only unknown entries is not enough: an archive that simply omits
// tables/expense_participants would restore cleanly and wipe every participant
// row, zeroing every balance in the instance with no error anywhere.
func (idx *Index) verifyComplete() error {
	if idx.Manifest.FormatVersion != FormatVersion {
		return fmt.Errorf("backup: manifest format version %d is not supported by this build (which reads version %d)", idx.Manifest.FormatVersion, FormatVersion)
	}
	for _, t := range InsertOrder() {
		parts := idx.TableParts[t.Name]
		if len(parts) == 0 {
			return fmt.Errorf("backup: archive is missing table %q; restoring it would silently empty that table", t.Name)
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].Part < parts[j].Part })
		for i, p := range parts {
			if p.Part != i {
				return fmt.Errorf("backup: table %q has a gap or duplicate in its parts (expected part %04d, found %04d)", t.Name, i, p.Part)
			}
		}
		idx.TableParts[t.Name] = parts
		if _, ok := idx.Manifest.RowCount(t.Name); !ok {
			return fmt.Errorf("backup: manifest does not record a row count for table %q", t.Name)
		}
	}
	for name := range idx.TableParts {
		if _, ok := LookupTable(name); !ok {
			return fmt.Errorf("backup: archive holds data for unknown table %q", name)
		}
	}
	for _, ts := range idx.Manifest.Tables {
		if _, ok := LookupTable(ts.Name); !ok {
			return fmt.Errorf("backup: manifest names unknown table %q", ts.Name)
		}
		if ts.RowCount < 0 {
			return fmt.Errorf("backup: manifest records a negative row count for table %q", ts.Name)
		}
	}
	if got := len(idx.Uploads); got != idx.Manifest.UploadFileCount {
		return fmt.Errorf("backup: archive holds %d upload files but the manifest records %d", got, idx.Manifest.UploadFileCount)
	}
	var total int64
	for _, u := range idx.Uploads {
		total += u.Size
	}
	if total != idx.Manifest.UploadTotalBytes {
		return fmt.Errorf("backup: archive uploads total %d bytes but the manifest records %d", total, idx.Manifest.UploadTotalBytes)
	}
	return nil
}

// hasPart reports whether Scan recorded this part for this table.
func (idx *Index) hasPart(table string, part int) bool {
	for _, p := range idx.TableParts[table] {
		if p.Part == part {
			return true
		}
	}
	return false
}

// ForEachRow makes the second forward pass, decoding rows and handing them to
// cb in InsertOrder. It trusts idx for what may appear and rejects anything
// that disagrees with it, so the two passes cannot diverge.
func ForEachRow(opener Opener, k *Keyring, hdr Header, aad []byte, limits Limits, idx *Index, cb func(t Table, row []json.RawMessage) error) error {
	rc, err := opener()
	if err != nil {
		return fmt.Errorf("backup: reopen archive: %w", err)
	}
	defer func() { _ = rc.Close() }()

	tr, _, err := openPayload(rc, k, hdr, aad, limits)
	if err != nil {
		return err
	}

	counts := map[string]int64{}
	for {
		th, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("backup: read archive payload: %w", err)
		}
		if th.Typeflag != tar.TypeReg || !strings.HasPrefix(th.Name, tablesPrefix) {
			continue
		}
		e, err := parseTablePart(th.Name)
		if err != nil {
			return err
		}
		t, ok := LookupTable(e.Table)
		if !ok {
			return fmt.Errorf("backup: archive holds data for unknown table %q", e.Table)
		}
		if !idx.hasPart(e.Table, e.Part) {
			return fmt.Errorf("backup: archive entry %q was not present when the archive was validated; retry the restore", th.Name)
		}
		n, err := decodeRows(tr, t, limits.MaxJSONLLineBytes, cb)
		if err != nil {
			return fmt.Errorf("backup: table %q part %04d: %w", e.Table, e.Part, err)
		}
		counts[e.Table] += n
	}

	// The manifest's counts are enforced, not advisory.
	for _, t := range InsertOrder() {
		want, _ := idx.Manifest.RowCount(t.Name)
		if counts[t.Name] != want {
			return fmt.Errorf("backup: table %q holds %d rows but the manifest records %d", t.Name, counts[t.Name], want)
		}
	}
	return nil
}

// decodeRows reads one table part's newline-delimited rows.
func decodeRows(r io.Reader, t Table, maxLine int, cb func(Table, []json.RawMessage) error) (int64, error) {
	sc := bufio.NewScanner(r)
	// An explicit bounded buffer, so an over-long line is a named error rather
	// than an opaque bufio.ErrTooLong. Some rows legitimately hold large JSON
	// (cached_bank_data.data), so the cap is configurable.
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var n int64
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		// json.RawMessage copies on unmarshal, so the decoded row does not
		// alias the scanner's buffer.
		var row []json.RawMessage
		if err := json.Unmarshal(line, &row); err != nil {
			return n, fmt.Errorf("row %d is not a JSON array: %w", n+1, err)
		}
		if len(row) != len(t.Columns) {
			return n, fmt.Errorf("row %d has %d values but table %q has %d columns", n+1, len(row), t.Name, len(t.Columns))
		}
		if err := cb(t, row); err != nil {
			return n, err
		}
		n++
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return n, fmt.Errorf("a row exceeds RESTORE_MAX_JSONL_LINE_MB (%d bytes)", maxLine)
		}
		return n, err
	}
	return n, nil
}

// openPayload unwraps the sealed, gzipped tar and returns a tar reader plus the
// counter measuring actual decompressed bytes.
func openPayload(r io.Reader, k *Keyring, hdr Header, aad []byte, limits Limits) (*tar.Reader, *countingReader, error) {
	prefix, perr := hdr.NoncePrefix()
	if perr != nil {
		return nil, nil, perr
	}
	// Skip the plaintext header on this fresh handle, and prove it is byte-for
	// byte the header the caller validated. Without this, a file swapped
	// between the two passes would be caught only indirectly, by the AEAD.
	_, raw, _, err := ReadHeader(r)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(raw, aad) {
		return nil, nil, errors.New("backup: the archive changed while it was being read; retry the restore")
	}
	opened, err := NewOpenReader(r, k, prefix, aad)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(opened)
	if err != nil {
		return nil, nil, fmt.Errorf("backup: archive payload is not valid gzip: %w", err)
	}
	counted := &countingReader{r: gz, max: limits.MaxTotalBytes}
	return tar.NewReader(counted), counted, nil
}

// countingReader measures bytes actually produced and trips the total cap, so
// a gzip bomb is stopped as it inflates rather than after.
type countingReader struct {
	r   io.Reader
	n   int64
	max int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.max > 0 && c.n > c.max {
		return n, fmt.Errorf("backup: archive expands past %d bytes (RESTORE_MAX_ARCHIVE_MB); refusing to continue", c.max)
	}
	return n, err
}

// parseTablePart validates a tables/ entry name and resolves it against the
// registry. The table name is only ever a lookup key here; it never reaches
// SQL text.
func parseTablePart(name string) (Entry, error) {
	rest := strings.TrimPrefix(name, tablesPrefix)
	tableName, file, ok := strings.Cut(rest, "/")
	if !ok || strings.Contains(file, "/") {
		return Entry{}, fmt.Errorf("backup: malformed table entry %q (expected %s<table>/<nnnn>.jsonl)", name, tablesPrefix)
	}
	if _, known := LookupTable(tableName); !known {
		return Entry{}, fmt.Errorf("backup: archive holds data for unknown table %q", tableName)
	}
	numStr, ok := strings.CutSuffix(file, ".jsonl")
	if !ok || len(numStr) != 4 {
		return Entry{}, fmt.Errorf("backup: malformed table part %q (expected a four-digit .jsonl name)", name)
	}
	part, err := strconv.Atoi(numStr)
	if err != nil || part < 0 {
		return Entry{}, fmt.Errorf("backup: malformed table part number in %q", name)
	}
	return Entry{Name: name, Kind: entryTablePart, Table: tableName, Part: part}, nil
}

// safeUploadRel validates an uploads/ entry name and returns its cleaned
// relative path.
//
// Tar names are always slash-separated, so a backslash or a colon is anomalous
// in itself -- a Windows separator or drive letter -- and is rejected outright
// rather than normalized. The path must already be canonical; anything that
// would change under Clean is refused instead of silently rewritten. This is
// deliberately OS-independent, so a restore run from a Windows shell is
// checked identically to one inside the Linux container.
func safeUploadRel(name string) (string, error) {
	rel := strings.TrimPrefix(name, uploadsPrefix)
	if rel == "" {
		return "", fmt.Errorf("backup: archive holds an empty upload path")
	}
	if strings.ContainsAny(rel, `\:`) {
		return "", fmt.Errorf("backup: upload path %q contains a backslash or colon", rel)
	}
	if strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("backup: upload path %q is absolute", rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		switch seg {
		case "":
			return "", fmt.Errorf("backup: upload path %q has an empty segment", rel)
		case ".", "..":
			return "", fmt.Errorf("backup: upload path %q escapes the upload directory", rel)
		}
	}
	if path.Clean(rel) != rel {
		return "", fmt.Errorf("backup: upload path %q is not canonical", rel)
	}
	return rel, nil
}

// extractUpload writes one upload into the staging directory, bounded by
// maxBytes. Archive mode bits are ignored: files land 0644 and directories
// 0755, so an archive cannot make anything executable or world-writable.
func extractUpload(r io.Reader, stagingDir, rel string, maxBytes int64) (int64, error) {
	if stagingDir == "" {
		return drainCapped(r, maxBytes)
	}
	dst := filepath.Join(stagingDir, filepath.FromSlash(rel))
	// Belt and braces: safeUploadRel already rejected anything that could
	// escape, and this re-checks the joined result on the real filesystem.
	root := filepath.Clean(stagingDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(dst)+string(os.PathSeparator), root) {
		return 0, fmt.Errorf("backup: upload path %q resolves outside the staging directory", rel)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, fmt.Errorf("backup: create %s: %w", filepath.Dir(dst), err)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return 0, fmt.Errorf("backup: create %s: %w", dst, err)
	}
	defer func() { _ = f.Close() }()

	n, err := io.Copy(f, io.LimitReader(r, maxBytes+1))
	if err != nil {
		return 0, fmt.Errorf("backup: write %s: %w", dst, err)
	}
	if n > maxBytes {
		return 0, fmt.Errorf("backup: upload %q exceeds the %d-byte per-file limit (UPLOAD_MAX_FILE_SIZE_MB)", rel, maxBytes)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("backup: close %s: %w", dst, err)
	}
	return n, nil
}

// readCapped reads at most max bytes, erroring if there are more.
func readCapped(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("entry exceeds the %d-byte limit", max)
	}
	return b, nil
}

// drainCapped reads and discards at most max bytes, erroring if there are more.
func drainCapped(r io.Reader, max int64) (int64, error) {
	n, err := io.Copy(io.Discard, io.LimitReader(r, max+1))
	if err != nil {
		return 0, err
	}
	if n > max {
		return 0, fmt.Errorf("entry exceeds the %d-byte limit", max)
	}
	return n, nil
}

// decodeManifest parses the sealed manifest strictly.
func decodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("backup: archive manifest is not valid: %w", err)
	}
	return m, nil
}
