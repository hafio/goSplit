package store

import (
	"context"
	"database/sql"
	"errors"
)

// GetBankData returns the cached bank data JSON for a user, or ErrNotFound.
func (s *Store) GetBankData(ctx context.Context, userID int64) (string, error) {
	var data string
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT data FROM cached_bank_data WHERE user_id = ?`), userID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return data, nil
}

// PutBankData stores/updates the cached bank data JSON for a user.
func (s *Store) PutBankData(ctx context.Context, userID int64, data string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO cached_bank_data (user_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`),
		userID, data, nowISO())
	return err
}
