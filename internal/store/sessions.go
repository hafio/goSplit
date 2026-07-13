package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CreateSession stores a session token for a user with the given TTL.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, ttl time.Duration) (*Session, error) {
	exp := FromNow(ttl)
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO sessions (token, user_id, expires) VALUES (?, ?, ?)`), token, userID, exp)
	if err != nil {
		return nil, err
	}
	return &Session{Token: token, UserID: userID, Expires: exp}, nil
}

// GetSession returns a non-expired session by token.
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	var sess Session
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT token, user_id, expires FROM sessions WHERE token = ?`), token).
		Scan(&sess.Token, &sess.UserID, &sess.Expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if sess.Expires < nowISO() {
		_ = s.DeleteSession(ctx, token)
		return nil, ErrNotFound
	}
	return &sess, nil
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM sessions WHERE token = ?`), token)
	return err
}

// DeleteUserSessions removes all sessions for a user (e.g. on password change).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM sessions WHERE user_id = ?`), userID)
	return err
}

// DeleteExpiredSessions purges expired sessions (scheduler cleanup).
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM sessions WHERE expires < ?`), nowISO())
	return err
}
