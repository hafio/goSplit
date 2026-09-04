package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/scrypt"
)

// Every archive is sealed (R5), with the key derived from the SESSION_SECRET
// the instance already requires. That has a consequence worth being blunt
// about in the docs: rotating or losing SESSION_SECRET makes every existing
// archive permanently unrestorable, and there is deliberately no override key.
// The fingerprint below is what turns that from a mystery into a clear message.

const (
	// scrypt parameters. A slow KDF is warranted even though SESSION_SECRET is
	// a secret rather than a user password: config only enforces a length of
	// 16, so a low-entropy passphrase is legal, and an archive is a
	// long-lived, offline-attackable artifact.
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1

	keyLen  = 32 // AES-256
	saltLen = 16

	// noncePrefixLen + 4-byte counter + 1-byte final flag == 12, the standard
	// GCM nonce length.
	noncePrefixLen = 7
	nonceLen       = 12

	// ChunkSize is the plaintext bytes sealed per frame. Framing rather than
	// one Seal over the whole payload keeps peak memory flat: an archive
	// carrying the upload tree has no useful size bound.
	ChunkSize = 1 << 20

	// maxFrameLen bounds a length read off the wire before any allocation.
	maxFrameLen = ChunkSize + 4096

	sealInfo        = "gosplit-backup-v1-seal"
	fingerprintInfo = "gosplit-backup-v1-fingerprint"
	fingerprintLbl  = "gosplit-backup-v1-fingerprint-label"
)

// ErrWrongSessionSecret is returned when an archive's recorded key fingerprint
// does not match the one derived from the running instance's SESSION_SECRET.
// It is checked before any decryption is attempted, so an operator gets this
// rather than a bare authentication failure from the cipher.
var ErrWrongSessionSecret = errors.New("backup: archive was sealed with a different SESSION_SECRET (restore it with the secret that created it; there is no override key)")

// ErrTruncatedArchive is returned when a sealed stream ends without a
// final-flagged frame, or carries trailing bytes after one.
var ErrTruncatedArchive = errors.New("backup: sealed payload is truncated or has trailing data")

// Keyring holds the subkeys derived from SESSION_SECRET for one archive. The
// sealing key stays unexported so it cannot reach a log or an error message by
// accident; only the fingerprint is ever published.
type Keyring struct {
	seal        []byte
	fingerprint []byte
}

// NewKeyring derives an archive's subkeys from sessionSecret and the archive's
// salt. One scrypt pass produces a master key, then HKDF splits it into
// domain-separated sealing and fingerprint keys -- so publishing the
// fingerprint cannot weaken the sealing key.
func NewKeyring(sessionSecret string, salt []byte) (*Keyring, error) {
	if sessionSecret == "" {
		return nil, errors.New("backup: SESSION_SECRET is empty; it is required to seal or open an archive")
	}
	if len(salt) != saltLen {
		return nil, fmt.Errorf("backup: salt must be %d bytes, got %d", saltLen, len(salt))
	}
	master, err := scrypt.Key([]byte(sessionSecret), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("backup: derive master key: %w", err)
	}
	sealKey, err := expand(master, sealInfo)
	if err != nil {
		return nil, err
	}
	fpKey, err := expand(master, fingerprintInfo)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, fpKey)
	_, _ = mac.Write([]byte(fingerprintLbl))
	return &Keyring{seal: sealKey, fingerprint: mac.Sum(nil)}, nil
}

// expand derives one subkey from the master key under an info label.
func expand(master []byte, info string) ([]byte, error) {
	out := make([]byte, keyLen)
	if _, err := io.ReadFull(hkdf.New(sha256.New, master, nil, []byte(info)), out); err != nil {
		return nil, fmt.Errorf("backup: derive %s subkey: %w", info, err)
	}
	return out, nil
}

// Fingerprint returns the archive's non-secret key fingerprint: an HMAC of a
// fixed public label under a subkey that is not the sealing key. It identifies
// which secret sealed an archive without revealing anything about it.
func (k *Keyring) Fingerprint() []byte {
	out := make([]byte, len(k.fingerprint))
	copy(out, k.fingerprint)
	return out
}

// FingerprintMatches compares a recorded fingerprint in constant time.
func (k *Keyring) FingerprintMatches(recorded []byte) bool {
	return hmac.Equal(k.fingerprint, recorded)
}

// NewSalt returns a fresh random salt for a new archive.
func NewSalt() ([]byte, error) { return randomBytes(saltLen) }

// NewNoncePrefix returns a fresh random nonce prefix for a new archive.
func NewNoncePrefix() ([]byte, error) { return randomBytes(noncePrefixLen) }

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("backup: read random bytes: %w", err)
	}
	return b, nil
}

// nonceFor builds the nonce for one frame. The chunk counter and the final
// flag are both inside it, which is what makes a reordered, duplicated,
// dropped or truncated stream an authentication failure rather than silently
// accepted partial data.
func nonceFor(prefix []byte, counter uint32, final bool) []byte {
	n := make([]byte, nonceLen)
	copy(n, prefix)
	binary.BigEndian.PutUint32(n[noncePrefixLen:], counter)
	if final {
		n[nonceLen-1] = 1
	}
	return n
}

// sealWriter frames and seals a plaintext stream.
//
// Wire format per frame: final flag (1 byte), sealed length (uint32 big
// endian), then the sealed bytes. The flag is repeated in the nonce, so
// flipping it in the header makes the frame fail to open.
type sealWriter struct {
	w       io.Writer
	aead    cipher.AEAD
	prefix  []byte
	aad     []byte
	buf     []byte
	counter uint32
	closed  bool
}

// NewSealWriter wraps w so that everything written to it is sealed in frames.
// Close must be called: it emits the final-flagged frame that marks the stream
// complete, and without it the reader reports a truncated archive.
func NewSealWriter(w io.Writer, k *Keyring, noncePrefix, aad []byte) (io.WriteCloser, error) {
	aead, err := newAEAD(k, noncePrefix)
	if err != nil {
		return nil, err
	}
	return &sealWriter{
		w:      w,
		aead:   aead,
		prefix: noncePrefix,
		aad:    aad,
		buf:    make([]byte, 0, ChunkSize),
	}, nil
}

func (s *sealWriter) Write(p []byte) (int, error) {
	if s.closed {
		return 0, errors.New("backup: write after close")
	}
	written := len(p)
	for len(p) > 0 {
		space := ChunkSize - len(s.buf)
		n := min(space, len(p))
		s.buf = append(s.buf, p[:n]...)
		p = p[n:]
		if len(s.buf) == ChunkSize {
			if err := s.flush(false); err != nil {
				return 0, err
			}
		}
	}
	return written, nil
}

// Close seals whatever is buffered as the final frame. An empty payload still
// produces one final frame, so every well-formed archive has a terminator.
func (s *sealWriter) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.flush(true)
}

func (s *sealWriter) flush(final bool) error {
	sealed := s.aead.Seal(nil, nonceFor(s.prefix, s.counter, final), s.buf, s.aad)
	if len(sealed) > maxFrameLen {
		return fmt.Errorf("backup: sealed frame of %d bytes exceeds the %d-byte limit", len(sealed), maxFrameLen)
	}
	var hdr [5]byte
	if final {
		hdr[0] = 1
	}
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(sealed)))
	if _, err := s.w.Write(hdr[:]); err != nil {
		return fmt.Errorf("backup: write frame header: %w", err)
	}
	if _, err := s.w.Write(sealed); err != nil {
		return fmt.Errorf("backup: write frame: %w", err)
	}
	s.counter++
	s.buf = s.buf[:0]
	return nil
}

// openReader reverses sealWriter.
type openReader struct {
	r       io.Reader
	aead    cipher.AEAD
	prefix  []byte
	aad     []byte
	buf     []byte
	counter uint32
	done    bool
	err     error
}

// NewOpenReader wraps a sealed stream, yielding the plaintext. It reports
// ErrTruncatedArchive if the stream ends without a final frame or carries
// trailing bytes after one, and a cipher authentication error if any frame was
// tampered with, reordered or dropped.
func NewOpenReader(r io.Reader, k *Keyring, noncePrefix, aad []byte) (io.Reader, error) {
	aead, err := newAEAD(k, noncePrefix)
	if err != nil {
		return nil, err
	}
	return &openReader{r: r, aead: aead, prefix: noncePrefix, aad: aad}, nil
}

func (o *openReader) Read(p []byte) (int, error) {
	for len(o.buf) == 0 {
		if o.err != nil {
			return 0, o.err
		}
		if o.done {
			// A final frame was consumed; nothing may follow it.
			var probe [1]byte
			if _, err := io.ReadFull(o.r, probe[:]); err == nil {
				o.err = ErrTruncatedArchive
			} else {
				o.err = io.EOF
			}
			return 0, o.err
		}
		if err := o.next(); err != nil {
			o.err = err
			return 0, err
		}
	}
	n := copy(p, o.buf)
	o.buf = o.buf[n:]
	return n, nil
}

func (o *openReader) next() error {
	var hdr [5]byte
	if _, err := io.ReadFull(o.r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// The stream ended without a final-flagged frame.
			return ErrTruncatedArchive
		}
		return fmt.Errorf("backup: read frame header: %w", err)
	}
	final := hdr[0] == 1
	if hdr[0] > 1 {
		return fmt.Errorf("backup: frame %d has an invalid flag byte %d", o.counter, hdr[0])
	}
	n := binary.BigEndian.Uint32(hdr[1:])
	if n == 0 || n > maxFrameLen {
		return fmt.Errorf("backup: frame %d declares an out-of-range length of %d bytes", o.counter, n)
	}
	sealed := make([]byte, n)
	if _, err := io.ReadFull(o.r, sealed); err != nil {
		return ErrTruncatedArchive
	}
	plain, err := o.aead.Open(nil, nonceFor(o.prefix, o.counter, final), sealed, o.aad)
	if err != nil {
		return fmt.Errorf("backup: frame %d failed authentication (the archive is corrupt, altered, or its frames were reordered): %w", o.counter, err)
	}
	o.counter++
	o.buf = plain
	o.done = final
	return nil
}

func newAEAD(k *Keyring, noncePrefix []byte) (cipher.AEAD, error) {
	if k == nil {
		return nil, errors.New("backup: nil keyring")
	}
	if len(noncePrefix) != noncePrefixLen {
		return nil, fmt.Errorf("backup: nonce prefix must be %d bytes, got %d", noncePrefixLen, len(noncePrefix))
	}
	block, err := aes.NewCipher(k.seal)
	if err != nil {
		return nil, fmt.Errorf("backup: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("backup: new GCM: %w", err)
	}
	return aead, nil
}
