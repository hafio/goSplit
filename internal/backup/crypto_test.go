package backup

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

// testKeyring builds a keyring with a fixed salt, so tests are deterministic.
func testKeyring(t *testing.T, secret string) *Keyring {
	t.Helper()
	salt := bytes.Repeat([]byte{0x2a}, saltLen)
	k, err := NewKeyring(secret, salt)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return k
}

func testPrefix() []byte { return bytes.Repeat([]byte{0x7}, noncePrefixLen) }

// seal is a helper that seals payload and returns the framed bytes.
func seal(t *testing.T, k *Keyring, aad, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewSealWriter(&buf, k, testPrefix(), aad)
	if err != nil {
		t.Fatalf("NewSealWriter: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

// open is a helper that opens sealed bytes.
func open(t *testing.T, k *Keyring, aad, sealed []byte) ([]byte, error) {
	t.Helper()
	r, err := NewOpenReader(bytes.NewReader(sealed), k, testPrefix(), aad)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// TestSealOpenRoundTrip covers payloads either side of the chunk boundary, so
// the multi-frame path is exercised rather than assumed.
func TestSealOpenRoundTrip(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("header-bytes")

	sizes := []int{0, 1, 1024, ChunkSize - 1, ChunkSize, ChunkSize + 1, 2*ChunkSize + 7}
	for _, n := range sizes {
		payload := make([]byte, n)
		if _, err := rand.Read(payload); err != nil {
			t.Fatalf("rand: %v", err)
		}
		sealed := seal(t, k, aad, payload)
		got, err := open(t, k, aad, sealed)
		if err != nil {
			t.Fatalf("size %d: open: %v", n, err)
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("size %d: round trip differs (%d bytes back)", n, len(got))
		}
	}
}

// TestEmptyPayloadStillTerminates asserts an empty payload produces a
// final-flagged frame, so an empty archive is not indistinguishable from a
// truncated one.
func TestEmptyPayloadStillTerminates(t *testing.T) {
	k := testKeyring(t, testSecret)
	sealed := seal(t, k, nil, nil)
	if len(sealed) == 0 {
		t.Fatal("an empty payload produced no frames at all")
	}
	if sealed[0] != 1 {
		t.Errorf("the single frame is not flagged final (flag byte = %d)", sealed[0])
	}
	got, err := open(t, k, nil, sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("opened %d bytes from an empty payload", len(got))
	}
}

// TestFingerprintDistinguishesSecrets asserts two different SESSION_SECRET
// values fingerprint differently under the same salt. This is what turns a
// rotated secret into an actionable message instead of a cipher failure.
func TestFingerprintDistinguishesSecrets(t *testing.T) {
	a := testKeyring(t, testSecret)
	b := testKeyring(t, "an-entirely-different-session-secret")

	if bytes.Equal(a.Fingerprint(), b.Fingerprint()) {
		t.Fatal("two different secrets produced the same fingerprint")
	}
	if !a.FingerprintMatches(a.Fingerprint()) {
		t.Error("a keyring does not match its own fingerprint")
	}
	if a.FingerprintMatches(b.Fingerprint()) {
		t.Error("a keyring matched another secret's fingerprint")
	}
}

// TestFingerprintIsStable asserts the same secret and salt always yield the
// same fingerprint -- otherwise a valid archive would be rejected.
func TestFingerprintIsStable(t *testing.T) {
	a := testKeyring(t, testSecret)
	b := testKeyring(t, testSecret)
	if !bytes.Equal(a.Fingerprint(), b.Fingerprint()) {
		t.Error("the same secret and salt produced different fingerprints")
	}
}

// TestFingerprintDoesNotRevealSealKey asserts the published fingerprint is not
// simply the sealing key, and that the two subkeys are domain-separated.
func TestFingerprintDoesNotRevealSealKey(t *testing.T) {
	k := testKeyring(t, testSecret)
	if bytes.Equal(k.Fingerprint(), k.seal) {
		t.Fatal("the fingerprint equals the sealing key; publishing it would leak the key")
	}
	if bytes.Contains(k.Fingerprint(), k.seal) {
		t.Fatal("the fingerprint contains the sealing key")
	}
}

// TestSaltChangesDerivedKey asserts the salt actually participates, so two
// archives from one secret do not share a sealing key.
func TestSaltChangesDerivedKey(t *testing.T) {
	s1 := bytes.Repeat([]byte{1}, saltLen)
	s2 := bytes.Repeat([]byte{2}, saltLen)
	k1, err := NewKeyring(testSecret, s1)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	k2, err := NewKeyring(testSecret, s2)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	if bytes.Equal(k1.seal, k2.seal) {
		t.Error("different salts produced the same sealing key")
	}
	if bytes.Equal(k1.Fingerprint(), k2.Fingerprint()) {
		t.Error("different salts produced the same fingerprint")
	}
}

// TestNewKeyringRejectsBadInput covers the argument guards.
func TestNewKeyringRejectsBadInput(t *testing.T) {
	if _, err := NewKeyring("", bytes.Repeat([]byte{1}, saltLen)); err == nil {
		t.Error("an empty secret was accepted")
	}
	if _, err := NewKeyring(testSecret, []byte{1, 2, 3}); err == nil {
		t.Error("a short salt was accepted")
	}
}

// TestOpenWithWrongKeyFails asserts the AEAD refuses a payload sealed under a
// different key, independently of the fingerprint pre-check.
func TestOpenWithWrongKeyFails(t *testing.T) {
	good := testKeyring(t, testSecret)
	bad := testKeyring(t, "another-session-secret-entirely-x")
	sealed := seal(t, good, []byte("aad"), []byte("secret payload"))

	if _, err := open(t, bad, []byte("aad"), sealed); err == nil {
		t.Fatal("a payload opened under the wrong key")
	}
}

// TestOpenWithWrongAADFails asserts the header is bound to the payload, so a
// header and a payload cannot be spliced from two different archives.
func TestOpenWithWrongAADFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	sealed := seal(t, k, []byte("header-A"), []byte("payload"))

	if _, err := open(t, k, []byte("header-B"), sealed); err == nil {
		t.Fatal("a payload opened under a different header; splicing is possible")
	}
}

// TestTamperedFrameFails asserts a flipped ciphertext bit is detected.
func TestTamperedFrameFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	sealed := seal(t, k, aad, bytes.Repeat([]byte("payload"), 100))

	// Flip a bit well inside the first frame's ciphertext, past the 5-byte
	// frame header.
	tampered := append([]byte(nil), sealed...)
	tampered[20] ^= 0x01

	if _, err := open(t, k, aad, tampered); err == nil {
		t.Fatal("a tampered frame opened successfully")
	}
}

// TestTruncatedStreamFails asserts a stream cut short is reported rather than
// yielding partial data. Silent truncation is the dangerous failure: it would
// look like a smaller but valid archive.
func TestTruncatedStreamFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	// Two chunks, so dropping the tail leaves a plausible-looking prefix.
	sealed := seal(t, k, aad, make([]byte, ChunkSize+500))

	for _, cut := range []int{len(sealed) - 1, len(sealed) / 2, 3} {
		_, err := open(t, k, aad, sealed[:cut])
		if err == nil {
			t.Errorf("a stream truncated to %d bytes opened successfully", cut)
		}
	}

	// Dropping exactly the final frame must also fail, not return the first
	// chunk as though it were the whole payload.
	flag, n := frameAt(t, sealed, 0)
	if flag != 0 {
		t.Fatalf("expected the first of two frames to be non-final, got flag %d", flag)
	}
	firstOnly := sealed[:5+int(n)]
	got, err := open(t, k, aad, firstOnly)
	if !errors.Is(err, ErrTruncatedArchive) {
		t.Errorf("dropping the final frame returned (%d bytes, %v), want ErrTruncatedArchive", len(got), err)
	}
}

// TestTrailingDataFails asserts bytes appended after the final frame are
// rejected.
func TestTrailingDataFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	sealed := seal(t, k, aad, []byte("payload"))
	extended := append(append([]byte(nil), sealed...), 0xff, 0xff, 0xff)

	if _, err := open(t, k, aad, extended); !errors.Is(err, ErrTruncatedArchive) {
		t.Errorf("trailing data returned %v, want ErrTruncatedArchive", err)
	}
}

// TestReorderedFramesFail asserts the per-frame counter in the nonce catches a
// swapped pair of chunks.
func TestReorderedFramesFail(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	payload := make([]byte, 2*ChunkSize+10)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}
	sealed := seal(t, k, aad, payload)

	// Frames are (flag, len, body). Swap the first two.
	_, n0 := frameAt(t, sealed, 0)
	f0end := 5 + int(n0)
	_, n1 := frameAt(t, sealed, f0end)
	f1end := f0end + 5 + int(n1)

	swapped := make([]byte, 0, len(sealed))
	swapped = append(swapped, sealed[f0end:f1end]...)
	swapped = append(swapped, sealed[:f0end]...)
	swapped = append(swapped, sealed[f1end:]...)

	if _, err := open(t, k, aad, swapped); err == nil {
		t.Fatal("reordered frames opened successfully")
	}
}

// TestDuplicatedFrameFails asserts replaying a frame is caught.
func TestDuplicatedFrameFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	sealed := seal(t, k, aad, make([]byte, ChunkSize+10))

	_, n0 := frameAt(t, sealed, 0)
	f0 := sealed[:5+int(n0)]
	dup := append(append([]byte(nil), f0...), sealed...)

	if _, err := open(t, k, aad, dup); err == nil {
		t.Fatal("a duplicated frame opened successfully")
	}
}

// TestFlippedFinalFlagFails asserts the final flag is authenticated, since it
// is part of the nonce.
func TestFlippedFinalFlagFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	aad := []byte("aad")
	sealed := seal(t, k, aad, []byte("payload"))

	flipped := append([]byte(nil), sealed...)
	if flipped[0] != 1 {
		t.Fatalf("expected a final first frame, got flag %d", flipped[0])
	}
	flipped[0] = 0

	if _, err := open(t, k, aad, flipped); err == nil {
		t.Fatal("clearing the final flag was not detected")
	}
}

// TestInvalidFrameHeaderRejected covers the length and flag guards that run
// before any allocation.
func TestInvalidFrameHeaderRejected(t *testing.T) {
	k := testKeyring(t, testSecret)

	tests := []struct {
		name  string
		frame []byte
	}{
		{"bad flag byte", append([]byte{9, 0, 0, 0, 16}, make([]byte, 16)...)},
		{"zero length", []byte{0, 0, 0, 0, 0}},
		{"absurd length", []byte{0, 0xff, 0xff, 0xff, 0xff}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := open(t, k, nil, tc.frame); err == nil {
				t.Error("an invalid frame header was accepted")
			}
		})
	}
}

// TestWriteAfterCloseFails asserts the seal writer refuses more data once the
// final frame has been emitted.
func TestWriteAfterCloseFails(t *testing.T) {
	k := testKeyring(t, testSecret)
	var buf bytes.Buffer
	w, err := NewSealWriter(&buf, k, testPrefix(), nil)
	if err != nil {
		t.Fatalf("NewSealWriter: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := w.Write([]byte("more")); err == nil {
		t.Error("writing after close succeeded")
	}
	if !strings.Contains(errString(w.Close()), "") {
		t.Error("a second close should be a no-op")
	}
}

// TestNoncePrefixLengthEnforced asserts a wrong-sized prefix is refused rather
// than silently padded, which would risk nonce reuse.
func TestNoncePrefixLengthEnforced(t *testing.T) {
	k := testKeyring(t, testSecret)
	var buf bytes.Buffer
	if _, err := NewSealWriter(&buf, k, []byte{1, 2}, nil); err == nil {
		t.Error("a short nonce prefix was accepted")
	}
	if _, err := NewOpenReader(&buf, k, bytes.Repeat([]byte{1}, noncePrefixLen+3), nil); err == nil {
		t.Error("a long nonce prefix was accepted")
	}
}

// TestNonceLayout documents and pins the nonce construction: prefix, then a
// big-endian counter, then the final flag, totalling the standard GCM length.
func TestNonceLayout(t *testing.T) {
	prefix := testPrefix()
	n := nonceFor(prefix, 0x01020304, true)
	if len(n) != nonceLen {
		t.Fatalf("nonce is %d bytes, want %d", len(n), nonceLen)
	}
	if !bytes.Equal(n[:noncePrefixLen], prefix) {
		t.Error("nonce does not start with the prefix")
	}
	if got := binary.BigEndian.Uint32(n[noncePrefixLen:]); got != 0x01020304 {
		t.Errorf("counter = %#x, want %#x", got, 0x01020304)
	}
	if n[nonceLen-1] != 1 {
		t.Error("final flag not set in the nonce")
	}
	if nonceFor(prefix, 0, false)[nonceLen-1] != 0 {
		t.Error("final flag set for a non-final frame")
	}
	// Distinct counters must give distinct nonces, or GCM's security is void.
	if bytes.Equal(nonceFor(prefix, 1, false), nonceFor(prefix, 2, false)) {
		t.Error("two counters produced the same nonce")
	}
}

// frameAt reads the flag and length of the frame starting at off.
func frameAt(t *testing.T, sealed []byte, off int) (byte, uint32) {
	t.Helper()
	if off+5 > len(sealed) {
		t.Fatalf("no frame header at offset %d", off)
	}
	return sealed[off], binary.BigEndian.Uint32(sealed[off+1 : off+5])
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
