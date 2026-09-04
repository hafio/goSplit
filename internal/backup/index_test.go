package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests craft archives by hand, because the point is to feed the parser
// things the real writer would never produce. A restore consumes a file that
// could come from anywhere, so every guard needs a case that actually trips it.

// craftEntry is one tar entry to place in a crafted payload.
type craftEntry struct {
	name     string
	body     []byte
	typeflag byte // zero means tar.TypeReg
	linkname string
}

// craftArchive assembles a complete archive file from explicit entries. mutate
// may adjust the entry list and the manifest before they are written.
func craftArchive(t *testing.T, k *Keyring, entries []craftEntry, man Manifest) []byte {
	t.Helper()

	hdr := Header{
		FormatVersion:        FormatVersion,
		CreatedAt:            "2026-09-04T08:00:00.000Z",
		SourceEngine:         "sqlite",
		GosplitVersion:       "v0.0.0-test",
		KDFAlgo:              "scrypt",
		KDFN:                 scryptN,
		KDFR:                 scryptR,
		KDFP:                 scryptP,
		SaltB64:              b64(bytes.Repeat([]byte{0x2a}, saltLen)),
		KeyFingerprintB64:    b64(k.Fingerprint()),
		AEADAlgo:             "AES-256-GCM",
		NoncePrefixB64:       b64(testPrefix()),
		AEADChunkSizeHint:    ChunkSize,
		SchemaMigrationsHint: man.SchemaMigrations,
	}

	var out bytes.Buffer
	aad, err := WriteHeader(&out, hdr)
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	sealed, err := NewSealWriter(&out, k, testPrefix(), aad)
	if err != nil {
		t.Fatalf("NewSealWriter: %v", err)
	}
	gz := gzip.NewWriter(sealed)
	tw := tar.NewWriter(gz)

	write := func(e craftEntry) {
		tf := e.typeflag
		if tf == 0 {
			tf = tar.TypeReg
		}
		th := &tar.Header{
			Name:     e.name,
			Mode:     0o600,
			Typeflag: tf,
			Linkname: e.linkname,
			ModTime:  time.Unix(0, 0).UTC(),
			Format:   tar.FormatPAX,
		}
		if tf == tar.TypeReg {
			th.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(th); err != nil {
			t.Fatalf("write tar header %q: %v", e.name, err)
		}
		if tf == tar.TypeReg && len(e.body) > 0 {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatalf("write tar body %q: %v", e.name, err)
			}
		}
	}

	for _, e := range entries {
		write(e)
	}
	raw, err := json.Marshal(man)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	write(craftEntry{name: ManifestEntry, body: raw})

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := sealed.Close(); err != nil {
		t.Fatalf("close seal: %v", err)
	}
	return out.Bytes()
}

// baselineEntries returns one empty part per registry table, which is what a
// dump of a completely empty database looks like.
func baselineEntries() []craftEntry {
	var out []craftEntry
	for _, tbl := range InsertOrder() {
		out = append(out, craftEntry{name: tablePartName(tbl.Name, 0)})
	}
	return out
}

// baselineManifest matches baselineEntries.
func baselineManifest() Manifest {
	m := Manifest{
		FormatVersion:  FormatVersion,
		CreatedAt:      "2026-09-04T08:00:00.000Z",
		SourceEngine:   "sqlite",
		GosplitVersion: "v0.0.0-test",
	}
	for _, tbl := range InsertOrder() {
		m.Tables = append(m.Tables, TableStat{Name: tbl.Name, RowCount: 0})
	}
	return m
}

// scanCrafted runs Scan over a crafted archive, staging uploads into a temp
// directory, and returns the index or the error.
func scanCrafted(t *testing.T, archive []byte, staging string) (*Index, error) {
	t.Helper()
	k := testKeyring(t, testSecret)
	hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	opener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(archive)), nil
	}
	limits := Limits{
		MaxTotalBytes:      1 << 28,
		MaxUploadFileBytes: 1 << 20,
		MaxTablePartBytes:  1 << 24,
		MaxEntries:         1000,
		MaxJSONLLineBytes:  1 << 16,
	}
	return Scan(opener, k, hdr, aad, limits, staging)
}

// TestScanAcceptsBaseline proves the crafting helpers produce something valid,
// so a failure in the tests below is about the mutation and not the scaffold.
func TestScanAcceptsBaseline(t *testing.T) {
	k := testKeyring(t, testSecret)
	archive := craftArchive(t, k, baselineEntries(), baselineManifest())
	idx, err := scanCrafted(t, archive, t.TempDir())
	if err != nil {
		t.Fatalf("baseline archive rejected: %v", err)
	}
	if len(idx.TableParts) != TableCount() {
		t.Errorf("indexed %d tables, want %d", len(idx.TableParts), TableCount())
	}
}

// TestScanRejectsMissingTable is the important one. Rejecting only unknown
// entries is not enough: an archive that simply omits a table would restore
// cleanly and leave that table empty. Omitting expense_participants would zero
// every balance in the instance without a single error.
func TestScanRejectsMissingTable(t *testing.T) {
	k := testKeyring(t, testSecret)
	for _, victim := range []string{"expense_participants", "users", "archived_expenses", "schema_migrations"} {
		t.Run(victim, func(t *testing.T) {
			var entries []craftEntry
			for _, e := range baselineEntries() {
				if !strings.HasPrefix(e.name, tablesPrefix+victim+"/") {
					entries = append(entries, e)
				}
			}
			archive := craftArchive(t, k, entries, baselineManifest())
			_, err := scanCrafted(t, archive, t.TempDir())
			if err == nil {
				t.Fatalf("an archive missing table %q was accepted", victim)
			}
			if !strings.Contains(err.Error(), victim) {
				t.Errorf("error does not name the missing table: %v", err)
			}
		})
	}
}

// TestScanRejectsDuplicateEntry asserts a repeated entry is refused, so there
// is never a question of which copy was validated and which applied.
func TestScanRejectsDuplicateEntry(t *testing.T) {
	k := testKeyring(t, testSecret)
	entries := append(baselineEntries(), craftEntry{name: tablePartName("users", 0)})
	archive := craftArchive(t, k, entries, baselineManifest())

	_, err := scanCrafted(t, archive, t.TempDir())
	if err == nil {
		t.Fatal("a duplicate entry was accepted")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention the duplicate: %v", err)
	}
}

// TestScanRejectsUnknownEntries covers both an unknown table and an entry
// outside the two expected prefixes.
func TestScanRejectsUnknownEntries(t *testing.T) {
	k := testKeyring(t, testSecret)
	tests := []struct {
		name  string
		entry craftEntry
	}{
		{"unknown table", craftEntry{name: tablesPrefix + "not_a_table/0000.jsonl"}},
		{"balance view", craftEntry{name: tablesPrefix + "balance_view/0000.jsonl"}},
		{"stray top level", craftEntry{name: "evil.sh", body: []byte("#!/bin/sh")}},
		{"nested stray", craftEntry{name: "etc/passwd", body: []byte("root")}},
		{"malformed part name", craftEntry{name: tablesPrefix + "users/notanumber.jsonl"}},
		{"part without directory", craftEntry{name: tablesPrefix + "users.jsonl"}},
		{"wrong part digits", craftEntry{name: tablesPrefix + "users/1.jsonl"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			archive := craftArchive(t, k, append(baselineEntries(), tc.entry), baselineManifest())
			if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
				t.Errorf("entry %q was accepted", tc.entry.name)
			}
		})
	}
}

// TestScanRejectsPathTraversal is the extraction guard. The checks are
// deliberately OS-independent, so a restore run from a Windows shell is
// refused the same way as one inside the Linux container.
func TestScanRejectsPathTraversal(t *testing.T) {
	k := testKeyring(t, testSecret)
	hostile := []string{
		uploadsPrefix + "../evil",
		uploadsPrefix + "../../etc/passwd",
		uploadsPrefix + "a/../../evil",
		uploadsPrefix + "/etc/passwd",
		uploadsPrefix + `..\evil`,
		uploadsPrefix + `C:\Windows\evil`,
		uploadsPrefix + "a/./b",
		uploadsPrefix + "a//b",
		uploadsPrefix + ".",
		uploadsPrefix + "..",
		// uploadsPrefix on its own is covered by
		// TestSafeUploadRelRejectsUnencodablePaths: archive/tar refuses to
		// encode a regular-file header with a trailing slash, so that input
		// cannot reach the parser through a crafted tar at all.
	}
	for _, name := range hostile {
		t.Run(name, func(t *testing.T) {
			staging := t.TempDir()
			archive := craftArchive(t, k,
				append(baselineEntries(), craftEntry{name: name, body: []byte("pwned")}),
				baselineManifest())

			if _, err := scanCrafted(t, archive, staging); err == nil {
				t.Fatalf("hostile upload path %q was accepted", name)
			}
			// Nothing may have escaped the staging root.
			assertNothingOutside(t, staging)
		})
	}
}

// TestScanRejectsNonRegularEntries asserts links and devices are refused --
// the classic way to make an extraction write somewhere else entirely.
func TestScanRejectsNonRegularEntries(t *testing.T) {
	k := testKeyring(t, testSecret)
	tests := []struct {
		name  string
		entry craftEntry
	}{
		{"symlink", craftEntry{name: uploadsPrefix + "link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"}},
		{"hardlink", craftEntry{name: uploadsPrefix + "hard", typeflag: tar.TypeLink, linkname: "/etc/passwd"}},
		{"char device", craftEntry{name: uploadsPrefix + "dev", typeflag: tar.TypeChar}},
		{"block device", craftEntry{name: uploadsPrefix + "blk", typeflag: tar.TypeBlock}},
		{"fifo", craftEntry{name: uploadsPrefix + "fifo", typeflag: tar.TypeFifo}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			staging := t.TempDir()
			archive := craftArchive(t, k, append(baselineEntries(), tc.entry), baselineManifest())
			if _, err := scanCrafted(t, archive, staging); err == nil {
				t.Errorf("a %s entry was accepted", tc.name)
			}
			assertNothingOutside(t, staging)
		})
	}
}

// TestScanAcceptsLegitimateUploads confirms the traversal guard is not so
// strict that real nested uploads are refused.
func TestScanAcceptsLegitimateUploads(t *testing.T) {
	k := testKeyring(t, testSecret)
	man := baselineManifest()
	man.UploadFileCount = 2
	man.UploadTotalBytes = int64(len("one") + len("two"))

	entries := append(baselineEntries(),
		craftEntry{name: uploadsPrefix + "a.png", body: []byte("one")},
		craftEntry{name: uploadsPrefix + "nested/deep/b.pdf", body: []byte("two")},
	)
	staging := t.TempDir()
	idx, err := scanCrafted(t, craftArchive(t, k, entries, man), staging)
	if err != nil {
		t.Fatalf("legitimate uploads rejected: %v", err)
	}
	if len(idx.Uploads) != 2 {
		t.Fatalf("indexed %d uploads, want 2", len(idx.Uploads))
	}
	got, err := os.ReadFile(filepath.Join(staging, "nested", "deep", "b.pdf"))
	if err != nil {
		t.Fatalf("read staged upload: %v", err)
	}
	if string(got) != "two" {
		t.Errorf("staged content = %q, want %q", got, "two")
	}
}

// TestScanEnforcesManifestCounts asserts the manifest is verified rather than
// trusted, in both directions.
func TestScanEnforcesManifestCounts(t *testing.T) {
	k := testKeyring(t, testSecret)

	t.Run("row count too high", func(t *testing.T) {
		man := baselineManifest()
		for i := range man.Tables {
			if man.Tables[i].Name == "users" {
				man.Tables[i].RowCount = 5
			}
		}
		// Scan itself only indexes; the row-count check happens in ForEachRow,
		// so verify the mismatch is caught there.
		archive := craftArchive(t, k, baselineEntries(), man)
		idx, err := scanCrafted(t, archive, t.TempDir())
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
		err = ForEachRow(opener, testKeyring(t, testSecret), hdr, aad,
			Limits{MaxTotalBytes: 1 << 28, MaxJSONLLineBytes: 1 << 16}, idx,
			func(Table, []json.RawMessage) error { return nil })
		if err == nil {
			t.Fatal("a manifest claiming more rows than the archive holds was accepted")
		}
		if !strings.Contains(err.Error(), "users") {
			t.Errorf("error does not name the table: %v", err)
		}
	})

	t.Run("missing table row count", func(t *testing.T) {
		man := baselineManifest()
		man.Tables = man.Tables[1:] // drop the first table's stat
		archive := craftArchive(t, k, baselineEntries(), man)
		if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
			t.Fatal("a manifest missing a table's row count was accepted")
		}
	})

	t.Run("upload count mismatch", func(t *testing.T) {
		man := baselineManifest()
		man.UploadFileCount = 3 // but no uploads are present
		archive := craftArchive(t, k, baselineEntries(), man)
		if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
			t.Fatal("a manifest overstating the upload count was accepted")
		}
	})

	t.Run("upload bytes mismatch", func(t *testing.T) {
		man := baselineManifest()
		man.UploadFileCount = 1
		man.UploadTotalBytes = 9999
		entries := append(baselineEntries(), craftEntry{name: uploadsPrefix + "a", body: []byte("tiny")})
		archive := craftArchive(t, k, entries, man)
		if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
			t.Fatal("a manifest overstating the upload bytes was accepted")
		}
	})

	t.Run("unknown table in manifest", func(t *testing.T) {
		man := baselineManifest()
		man.Tables = append(man.Tables, TableStat{Name: "not_a_table", RowCount: 1})
		archive := craftArchive(t, k, baselineEntries(), man)
		if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
			t.Fatal("a manifest naming an unknown table was accepted")
		}
	})

	t.Run("negative row count", func(t *testing.T) {
		man := baselineManifest()
		man.Tables[0].RowCount = -1
		archive := craftArchive(t, k, baselineEntries(), man)
		if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
			t.Fatal("a negative row count was accepted")
		}
	})
}

// TestScanRejectsWrongFormatVersion asserts an archive from a future build is
// refused rather than misread.
func TestScanRejectsWrongFormatVersion(t *testing.T) {
	k := testKeyring(t, testSecret)
	man := baselineManifest()
	man.FormatVersion = FormatVersion + 1
	archive := craftArchive(t, k, baselineEntries(), man)

	if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
		t.Fatal("a manifest with a future format version was accepted")
	}
}

// TestScanEnforcesEntryCap asserts the entry-count limit trips, so an archive
// with a huge number of tiny entries cannot exhaust the process.
func TestScanEnforcesEntryCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	entries := baselineEntries()
	for i := 0; i < 200; i++ {
		entries = append(entries, craftEntry{
			name: uploadsPrefix + "f" + strings.Repeat("0", 3) + itoa(i),
			body: []byte("x"),
		})
	}
	archive := craftArchive(t, k, entries, baselineManifest())

	hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
	limits := Limits{
		MaxTotalBytes:      1 << 28,
		MaxUploadFileBytes: 1 << 20,
		MaxTablePartBytes:  1 << 24,
		MaxEntries:         50, // below the entry count above
		MaxJSONLLineBytes:  1 << 16,
	}
	if _, err := Scan(opener, k, hdr, aad, limits, t.TempDir()); err == nil {
		t.Fatal("the entry-count cap did not trip")
	} else if !strings.Contains(err.Error(), "RESTORE_MAX_ENTRIES") {
		t.Errorf("error should name the setting that tripped: %v", err)
	}
}

// TestScanEnforcesTotalByteCap asserts the cap is measured from bytes actually
// produced, which is what makes it a real defence against a gzip bomb -- a
// claimed size costs an attacker nothing to lie about.
func TestScanEnforcesTotalByteCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	// Highly compressible: 8 MiB of zeros shrinks to a few KiB.
	bomb := make([]byte, 8<<20)
	man := baselineManifest()
	man.UploadFileCount = 1
	man.UploadTotalBytes = int64(len(bomb))
	entries := append(baselineEntries(), craftEntry{name: uploadsPrefix + "bomb", body: bomb})
	archive := craftArchive(t, k, entries, man)

	if len(archive) > 1<<20 {
		t.Fatalf("crafted bomb did not compress as expected (%d bytes)", len(archive))
	}

	hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
	limits := Limits{
		MaxTotalBytes:      1 << 20, // far below the inflated size
		MaxUploadFileBytes: 32 << 20,
		MaxTablePartBytes:  1 << 24,
		MaxEntries:         1000,
		MaxJSONLLineBytes:  1 << 16,
	}
	if _, err := Scan(opener, k, hdr, aad, limits, t.TempDir()); err == nil {
		t.Fatal("an archive inflating past the total cap was accepted")
	}
}

// TestScanEnforcesPerUploadCap asserts one oversized upload is refused.
func TestScanEnforcesPerUploadCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	big := bytes.Repeat([]byte("a"), 2<<20)
	man := baselineManifest()
	man.UploadFileCount = 1
	man.UploadTotalBytes = int64(len(big))
	entries := append(baselineEntries(), craftEntry{name: uploadsPrefix + "big", body: big})
	archive := craftArchive(t, k, entries, man)

	// scanCrafted uses MaxUploadFileBytes of 1 MiB.
	if _, err := scanCrafted(t, archive, t.TempDir()); err == nil {
		t.Fatal("an oversized upload was accepted")
	}
}

// TestForEachRowRejectsMalformedRows covers the row-shape guards, which run on
// data the archive fully controls.
func TestForEachRowRejectsMalformedRows(t *testing.T) {
	k := testKeyring(t, testSecret)

	tests := []struct {
		name string
		line string
	}{
		{"not an array", `{"id":"1"}`},
		{"too few values", `["1"]`},
		{"too many values", `["1","2","3","4","5","6","7","8","9"]`},
		{"not json at all", `garbage`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var entries []craftEntry
			for _, e := range baselineEntries() {
				if e.name == tablePartName("app_metadata", 0) {
					e.body = []byte(tc.line + "\n")
				}
				entries = append(entries, e)
			}
			man := baselineManifest()
			for i := range man.Tables {
				if man.Tables[i].Name == "app_metadata" {
					man.Tables[i].RowCount = 1
				}
			}
			archive := craftArchive(t, k, entries, man)
			idx, err := scanCrafted(t, archive, t.TempDir())
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
			if err != nil {
				t.Fatalf("read header: %v", err)
			}
			opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
			err = ForEachRow(opener, testKeyring(t, testSecret), hdr, aad,
				Limits{MaxTotalBytes: 1 << 28, MaxJSONLLineBytes: 1 << 16}, idx,
				func(Table, []json.RawMessage) error { return nil })
			if err == nil {
				t.Errorf("malformed row %q was accepted", tc.line)
			}
		})
	}
}

// TestForEachRowEnforcesLineCap asserts an over-long row produces a named
// error rather than an opaque bufio failure.
func TestForEachRowEnforcesLineCap(t *testing.T) {
	k := testKeyring(t, testSecret)
	huge := `["` + strings.Repeat("x", 200000) + `","v"]`

	var entries []craftEntry
	for _, e := range baselineEntries() {
		if e.name == tablePartName("app_metadata", 0) {
			e.body = []byte(huge + "\n")
		}
		entries = append(entries, e)
	}
	man := baselineManifest()
	for i := range man.Tables {
		if man.Tables[i].Name == "app_metadata" {
			man.Tables[i].RowCount = 1
		}
	}
	archive := craftArchive(t, k, entries, man)
	idx, err := scanCrafted(t, archive, t.TempDir())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	hdr, aad, _, err := ReadHeader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	opener := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(archive)), nil }
	err = ForEachRow(opener, testKeyring(t, testSecret), hdr, aad,
		Limits{MaxTotalBytes: 1 << 28, MaxJSONLLineBytes: 4096}, idx,
		func(Table, []json.RawMessage) error { return nil })
	if err == nil {
		t.Fatal("an over-long row was accepted")
	}
	if !strings.Contains(err.Error(), "RESTORE_MAX_JSONL_LINE_MB") {
		t.Errorf("error should name the setting that tripped: %v", err)
	}
}

// TestReadHeaderRejectsBadContainers covers the outer-format guards, all of
// which run before a key is derived.
func TestReadHeaderRejectsBadContainers(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"too short", []byte("GSP")},
		{"bad magic", append([]byte("NOTAMAGIC"), make([]byte, 32)...)},
		{"zero header length", append([]byte(magic), append([]byte{FormatVersion}, 0, 0, 0, 0)...)},
		{"header not json", append([]byte(magic), append([]byte{FormatVersion}, 0, 0, 0, 4, 'j', 'u', 'n', 'k')...)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := ReadHeader(bytes.NewReader(tc.data)); err == nil {
				t.Error("a malformed container was accepted")
			}
		})
	}
}

// TestHeaderRejectsAbsurdKDFParameters asserts the scrypt cost is bounded.
// Without this, opening a hostile file could be turned into a memory-
// exhaustion denial of service by declaring an enormous N.
func TestHeaderRejectsAbsurdKDFParameters(t *testing.T) {
	base := Header{
		FormatVersion:     FormatVersion,
		KDFAlgo:           "scrypt",
		AEADAlgo:          "AES-256-GCM",
		KDFN:              scryptN,
		KDFR:              scryptR,
		KDFP:              scryptP,
		SaltB64:           b64(bytes.Repeat([]byte{1}, saltLen)),
		NoncePrefixB64:    b64(testPrefix()),
		KeyFingerprintB64: b64(bytes.Repeat([]byte{2}, 32)),
	}
	if err := base.validate(); err != nil {
		t.Fatalf("baseline header rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Header)
	}{
		{"huge N", func(h *Header) { h.KDFN = 1 << 30 }},
		{"tiny N", func(h *Header) { h.KDFN = 2 }},
		{"non power of two N", func(h *Header) { h.KDFN = 30000 }},
		{"huge r", func(h *Header) { h.KDFR = 1 << 20 }},
		{"zero r", func(h *Header) { h.KDFR = 0 }},
		{"huge p", func(h *Header) { h.KDFP = 1 << 20 }},
		{"zero p", func(h *Header) { h.KDFP = 0 }},
		{"unknown kdf", func(h *Header) { h.KDFAlgo = "rot13" }},
		{"unknown aead", func(h *Header) { h.AEADAlgo = "rot13" }},
		{"short salt", func(h *Header) { h.SaltB64 = b64([]byte{1, 2}) }},
		{"bad base64 salt", func(h *Header) { h.SaltB64 = "!!!not base64!!!" }},
		{"short nonce prefix", func(h *Header) { h.NoncePrefixB64 = b64([]byte{1}) }},
		{"short fingerprint", func(h *Header) { h.KeyFingerprintB64 = b64([]byte{1}) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := base
			tc.mutate(&h)
			if err := h.validate(); err == nil {
				t.Error("an invalid header was accepted")
			}
		})
	}
}

// TestSafeUploadRelAcceptsOrdinaryPaths pins the positive side of the path
// rule, so the guard cannot be tightened into rejecting real uploads.
func TestSafeUploadRelAcceptsOrdinaryPaths(t *testing.T) {
	good := []string{"a.png", "nested/b.pdf", "a/b/c/d.txt", "with space.png", "with-dash_and.dot.png"}
	for _, rel := range good {
		got, err := safeUploadRel(uploadsPrefix + rel)
		if err != nil {
			t.Errorf("safeUploadRel(%q) rejected a legitimate path: %v", rel, err)
			continue
		}
		if got != rel {
			t.Errorf("safeUploadRel(%q) = %q", rel, got)
		}
	}
}

// TestSafeUploadRelRejectsUnencodablePaths covers the path guard directly, for
// inputs archive/tar will not let a test express as a crafted entry. The guard
// still has to hold: a hand-rolled or non-Go tar writer is under no such
// constraint.
func TestSafeUploadRelRejectsUnencodablePaths(t *testing.T) {
	hostile := []string{
		uploadsPrefix,            // trailing slash, so an empty relative path
		uploadsPrefix + "a/",     // directory-shaped
		uploadsPrefix + "a//",    // empty trailing segment
		uploadsPrefix + `\`,      // a bare backslash
		uploadsPrefix + `a\b`,    // Windows separator
		uploadsPrefix + `..\a`,   // Windows-style traversal
		uploadsPrefix + "c:/x",   // drive letter
		uploadsPrefix + `C:\x`,   // drive letter with separator
		uploadsPrefix + "a/../b", // non-canonical
	}
	for _, name := range hostile {
		if _, err := safeUploadRel(name); err == nil {
			t.Errorf("safeUploadRel(%q) accepted a path it must reject", name)
		}
	}
}

// assertNothingOutside checks that a rejected extraction left no file above
// the staging root.
func assertNothingOutside(t *testing.T, staging string) {
	t.Helper()
	parent := filepath.Dir(staging)
	for _, name := range []string{"evil", "pwned", "passwd", "link", "hard"} {
		if _, err := os.Stat(filepath.Join(parent, name)); err == nil {
			t.Errorf("extraction escaped the staging root: %s exists in %s", name, parent)
		}
	}
}

// itoa avoids pulling strconv into the test for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
