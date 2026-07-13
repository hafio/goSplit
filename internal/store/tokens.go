package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// CreateVerificationToken stores a token for an identifier (email) and purpose.
func (s *Store) CreateVerificationToken(ctx context.Context, identifier, token, purpose string, ttl time.Duration) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO verification_tokens (identifier, token, purpose, expires) VALUES (?, ?, ?, ?)`),
		strings.ToLower(strings.TrimSpace(identifier)), token, purpose, FromNow(ttl))
	return err
}

// UseVerificationToken looks up a token, deletes it (single-use), and returns
// its identifier if it exists, matches purpose, and has not expired.
func (s *Store) UseVerificationToken(ctx context.Context, token, purpose string) (identifier string, err error) {
	var expires string
	err = s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT identifier, expires FROM verification_tokens WHERE token = ? AND purpose = ?`),
		token, purpose).Scan(&identifier, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	// Single-use: consume regardless of expiry.
	_, _ = s.DB.ExecContext(ctx, s.rebind(`DELETE FROM verification_tokens WHERE token = ?`), token)
	if expires < nowISO() {
		return "", ErrNotFound
	}
	return identifier, nil
}

// DeleteExpiredTokens purges expired verification tokens.
func (s *Store) DeleteExpiredTokens(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM verification_tokens WHERE expires < ?`), nowISO())
	return err
}
