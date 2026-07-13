package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// AcquireLock attempts to acquire or renew a named leader lock for holder with
// the given TTL. It returns true if holder now owns the lock. Uses an upsert
// that only overwrites an expired lock or one already held by holder — portable
// across SQLite and Postgres.
func (s *Store) AcquireLock(ctx context.Context, id, holder string, ttl time.Duration) (bool, error) {
	now := nowISO()
	expires := FromNow(ttl)
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO scheduler_locks (id, holder, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET holder = excluded.holder, expires_at = excluded.expires_at
		 WHERE scheduler_locks.expires_at < ? OR scheduler_locks.holder = ?`),
		id, holder, expires, now, holder)
	if err != nil {
		return false, err
	}
	var current string
	err = s.DB.QueryRowContext(ctx, s.rebind(`SELECT holder FROM scheduler_locks WHERE id = ?`), id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return current == holder, nil
}
