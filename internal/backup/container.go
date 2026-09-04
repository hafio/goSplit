package backup

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// The archive container. Outer-to-inner:
//
//	magic          "GSPLTBAK"       8 bytes
//	format_version uint8            1 byte
//	header_len     uint32 big endian
//	header         header_len bytes of plaintext JSON
//	payload        framed sealed frames (see crypto.go)
//
// The header is plaintext deliberately: a restore identifies the format, the
// engine and the key fingerprint, and fails fast, before deriving a key or
// attempting a single decryption. Its exact bytes are the AEAD associated data
// for every frame, so a header and a payload cannot be spliced together from
// two different archives.
//
// The sealed payload decompresses (gzip) to a tar holding, in order:
//
//	manifest.json                     the authoritative record
//	tables/<name>/<nnnn>.jsonl        row data, one or more parts per table
//	uploads/<path under UploadDir>    the upload tree, regular files only
//
// Table data is split into bounded parts because a tar entry's size must be
// known before its bytes are written. Parts keep the dump streaming with flat
// memory instead of buffering a whole table, which has no useful size bound.

const (
	magic = "GSPLTBAK"

	// FormatVersion is the container version this build writes, and the only
	// one it reads. A reader refuses anything else rather than guessing.
	FormatVersion = 1

	// ArchiveExt is the archive file extension.
	ArchiveExt = ".gsbak"

	// ManifestEntry is the tar entry holding the authoritative manifest.
	ManifestEntry = "manifest.json"

	// tablesPrefix and uploadsPrefix are the only two top-level tar prefixes.
	tablesPrefix  = "tables/"
	uploadsPrefix = "uploads/"

	// tablePartMaxBytes bounds one table part, and so bounds the dump's peak
	// memory per table regardless of row count.
	tablePartMaxBytes = 8 << 20

	// maxHeaderLen bounds the plaintext header before any allocation.
	maxHeaderLen = 1 << 20
)

// Header is the plaintext container header. It must never carry a secret: it
// is readable by anyone holding the file. The fingerprint is safe here by
// construction (see Keyring.Fingerprint).
type Header struct {
	FormatVersion  int    `json:"format_version"`
	CreatedAt      string `json:"created_at"`
	SourceEngine   string `json:"source_engine"`
	GosplitVersion string `json:"gosplit_version"`

	KDFAlgo string `json:"kdf_algo"`
	KDFN    int    `json:"kdf_n"`
	KDFR    int    `json:"kdf_r"`
	KDFP    int    `json:"kdf_p"`
	SaltB64 string `json:"kdf_salt_b64"`

	KeyFingerprintB64 string `json:"key_fingerprint_b64"`

	AEADAlgo          string `json:"aead_algo"`
	NoncePrefixB64    string `json:"aead_nonce_prefix_b64"`
	AEADChunkSizeHint int    `json:"aead_chunk_size"`

	// SchemaMigrationsHint lets `gosplit inspect` and the admin preview show
	// the schema set without a key. It is advisory only: the authoritative
	// check always uses the sealed manifest.
	SchemaMigrationsHint []string `json:"schema_migrations_hint"`
}

// TableStat records how many rows a dump wrote for one table. Verified on
// restore, not advisory: a mismatch rolls the whole restore back.
type TableStat struct {
	Name     string `json:"name"`
	RowCount int64  `json:"row_count"`
}

// Manifest is the authoritative record, carried inside the sealed payload.
type Manifest struct {
	FormatVersion  int    `json:"format_version"`
	CreatedAt      string `json:"created_at"`
	SourceEngine   string `json:"source_engine"`
	GosplitVersion string `json:"gosplit_version"`

	// SchemaMigrations is the sorted set of applied migration filenames at dump
	// time. It deliberately excludes applied_at: that column records when a
	// particular database first saw a migration, not the migration's identity,
	// so comparing it would make every restore onto a freshly provisioned
	// database -- the primary disaster-recovery case -- fail as a mismatch.
	SchemaMigrations []string `json:"schema_migrations"`

	Tables           []TableStat `json:"tables"`
	UploadFileCount  int         `json:"upload_file_count"`
	UploadTotalBytes int64       `json:"upload_total_bytes"`
}

// RowCount returns the recorded row count for a table.
func (m Manifest) RowCount(table string) (int64, bool) {
	for _, t := range m.Tables {
		if t.Name == table {
			return t.RowCount, true
		}
	}
	return 0, false
}

// Salt decodes the archive's KDF salt.
func (h Header) Salt() ([]byte, error) { return decodeB64("kdf_salt_b64", h.SaltB64, saltLen) }

// NoncePrefix decodes the archive's nonce prefix.
func (h Header) NoncePrefix() ([]byte, error) {
	return decodeB64("aead_nonce_prefix_b64", h.NoncePrefixB64, noncePrefixLen)
}

// KeyFingerprint decodes the archive's recorded key fingerprint.
func (h Header) KeyFingerprint() ([]byte, error) {
	return decodeB64("key_fingerprint_b64", h.KeyFingerprintB64, 32)
}

func decodeB64(field, v string, want int) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("backup: archive header %s is not valid base64: %w", field, err)
	}
	if len(b) != want {
		return nil, fmt.Errorf("backup: archive header %s must decode to %d bytes, got %d", field, want, len(b))
	}
	return b, nil
}

// validate checks the header's self-consistency before it is trusted for key
// derivation. It does not touch the payload.
func (h Header) validate() error {
	if h.FormatVersion != FormatVersion {
		return fmt.Errorf("backup: archive format version %d is not supported by this build (which reads version %d); restore it with the build that wrote it", h.FormatVersion, FormatVersion)
	}
	if h.KDFAlgo != "scrypt" {
		return fmt.Errorf("backup: unsupported key-derivation algorithm %q", h.KDFAlgo)
	}
	if h.AEADAlgo != "AES-256-GCM" {
		return fmt.Errorf("backup: unsupported encryption algorithm %q", h.AEADAlgo)
	}
	// The KDF parameters come from the archive, so they must be bounded: an
	// absurd N would otherwise turn opening a hostile file into a memory-
	// exhaustion denial of service.
	if h.KDFN < 1<<12 || h.KDFN > 1<<20 || h.KDFN&(h.KDFN-1) != 0 {
		return fmt.Errorf("backup: archive kdf_n %d is out of the accepted range (a power of two between 4096 and 1048576)", h.KDFN)
	}
	if h.KDFR < 1 || h.KDFR > 32 {
		return fmt.Errorf("backup: archive kdf_r %d is out of the accepted range (1 to 32)", h.KDFR)
	}
	if h.KDFP < 1 || h.KDFP > 16 {
		return fmt.Errorf("backup: archive kdf_p %d is out of the accepted range (1 to 16)", h.KDFP)
	}
	if _, err := h.Salt(); err != nil {
		return err
	}
	if _, err := h.NoncePrefix(); err != nil {
		return err
	}
	if _, err := h.KeyFingerprint(); err != nil {
		return err
	}
	return nil
}

// WriteHeader writes the magic, version and plaintext header, returning the
// exact header bytes so the caller can use them as the payload's AEAD
// associated data.
func WriteHeader(w io.Writer, h Header) ([]byte, error) {
	raw, err := json.Marshal(h)
	if err != nil {
		return nil, fmt.Errorf("backup: encode header: %w", err)
	}
	if len(raw) > maxHeaderLen {
		return nil, fmt.Errorf("backup: header of %d bytes exceeds the %d-byte limit", len(raw), maxHeaderLen)
	}
	var prefix [8 + 1 + 4]byte
	copy(prefix[:8], magic)
	prefix[8] = FormatVersion
	binary.BigEndian.PutUint32(prefix[9:], uint32(len(raw)))
	if _, err := w.Write(prefix[:]); err != nil {
		return nil, fmt.Errorf("backup: write header prefix: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return nil, fmt.Errorf("backup: write header: %w", err)
	}
	return raw, nil
}

// ReadHeader reads and validates the plaintext header without needing a key.
// It returns the parsed header, its exact bytes (the payload's associated
// data) and a reader positioned at the start of the sealed payload.
func ReadHeader(r io.Reader) (Header, []byte, io.Reader, error) {
	var prefix [8 + 1 + 4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return Header{}, nil, nil, fmt.Errorf("backup: this file is too short to be a %s archive: %w", ArchiveExt, err)
	}
	if string(prefix[:8]) != magic {
		return Header{}, nil, nil, fmt.Errorf("backup: this file is not a GoSplit archive (bad magic); expected a %s file written by `gosplit backup`", ArchiveExt)
	}
	if prefix[8] != FormatVersion {
		return Header{}, nil, nil, fmt.Errorf("backup: archive format version %d is not supported by this build (which reads version %d); restore it with the build that wrote it", prefix[8], FormatVersion)
	}
	n := binary.BigEndian.Uint32(prefix[9:])
	if n == 0 || n > maxHeaderLen {
		return Header{}, nil, nil, fmt.Errorf("backup: archive header declares an out-of-range length of %d bytes", n)
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(r, raw); err != nil {
		return Header{}, nil, nil, fmt.Errorf("backup: read archive header: %w", err)
	}
	var h Header
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return Header{}, nil, nil, fmt.Errorf("backup: archive header is not valid: %w", err)
	}
	if err := h.validate(); err != nil {
		return Header{}, nil, nil, err
	}
	return h, raw, r, nil
}

// tablePartName builds the tar entry name for one part of a table's rows.
// The table name is a validated registry identifier, so it can never contain a
// separator.
func tablePartName(table string, part int) string {
	return fmt.Sprintf("%s%s/%04d.jsonl", tablesPrefix, table, part)
}
