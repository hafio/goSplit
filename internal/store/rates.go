package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// GetCachedRate returns a cached rate string for (from,to,date), or ErrNotFound.
func (s *Store) GetCachedRate(ctx context.Context, from, to, date string) (string, error) {
	var rate string
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT rate FROM cached_currency_rates WHERE from_currency = ? AND to_currency = ? AND rate_date = ?`),
		strings.ToUpper(from), strings.ToUpper(to), date).Scan(&rate)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return rate, nil
}

// PutCachedRate stores/updates a rate for (from,to,date).
func (s *Store) PutCachedRate(ctx context.Context, from, to, date, rate string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO cached_currency_rates (from_currency, to_currency, rate_date, rate)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(from_currency, to_currency, rate_date) DO UPDATE SET rate = excluded.rate`),
		strings.ToUpper(from), strings.ToUpper(to), date, rate)
	return err
}

// DeleteRatesBefore purges cached rates older than the cutoff date (cleanup).
func (s *Store) DeleteRatesBefore(ctx context.Context, cutoffDate string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM cached_currency_rates WHERE rate_date < ?`), cutoffDate)
	return err
}
