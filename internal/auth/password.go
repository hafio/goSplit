// Package auth provides password hashing, session tokens, CSRF protection, and
// request authentication middleware. No external auth vendor: argon2id for
// passwords, signed random tokens for sessions and magic-links.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP-reasonable defaults for a self-hosted app).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

var (
	// ErrInvalidHash is returned when a stored hash cannot be parsed.
	ErrInvalidHash = errors.New("auth: invalid password hash format")
	// ErrMismatch is returned when a password does not match its hash.
	ErrMismatch = errors.New("auth: password does not match")
)

// HashPassword returns an encoded argon2id hash (PHC string format).
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("auth: empty password")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks a password against an encoded argon2id hash in constant
// time. It returns nil on match, ErrMismatch on mismatch, or ErrInvalidHash.
func VerifyPassword(password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return ErrInvalidHash
	}
	var mem uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return ErrInvalidHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return ErrInvalidHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return ErrInvalidHash
	}
	got := argon2.IDKey([]byte(password), salt, time, mem, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) == 1 {
		return nil
	}
	return ErrMismatch
}

// RandomToken returns a URL-safe random token with n bytes of entropy.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
