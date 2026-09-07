package store

import (
	"context"
	"database/sql"
	"errors"
)

const notificationCols = `id, user_id, actor_id, kind, entity_type, entity_id,
	title, amount, currency, read_at, created_at`

func scanNotification(row interface{ Scan(...any) error }) (*Notification, error) {
	var n Notification
	err := row.Scan(&n.ID, &n.UserID, &n.ActorID, &n.Kind, &n.EntityType, &n.EntityID,
		&n.Title, &n.Amount, &n.Currency, &n.ReadAt, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// CreateNotification inserts a notification and returns it with its id.
func (s *Store) CreateNotification(ctx context.Context, n *Notification) (*Notification, error) {
	id, err := s.insertReturningID(ctx,
		`INSERT INTO notifications (user_id, actor_id, kind, entity_type, entity_id,
		 title, amount, currency, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "notifications",
		n.UserID, n.ActorID, n.Kind, n.EntityType, n.EntityID,
		n.Title, n.Amount, n.Currency, nowISO())
	if err != nil {
		return nil, err
	}
	return s.GetNotification(ctx, id, n.UserID)
}

// CreateNotifications inserts a fan-out in one transaction, so notifying every
// participant of an expense is one commit rather than one per recipient.
func (s *Store) CreateNotifications(ctx context.Context, ns []*Notification) error {
	if len(ns) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := nowISO()
	for _, n := range ns {
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO notifications (user_id, actor_id, kind, entity_type, entity_id,
			 title, amount, currency, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			n.UserID, n.ActorID, n.Kind, n.EntityType, n.EntityID,
			n.Title, n.Amount, n.Currency, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetNotification returns one notification, scoped to its owner: userID is part
// of the WHERE clause, never checked afterwards, so a guessed id belonging to
// somebody else is indistinguishable from one that does not exist.
func (s *Store) GetNotification(ctx context.Context, id, userID int64) (*Notification, error) {
	return scanNotification(s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT `+notificationCols+` FROM notifications WHERE id = ? AND user_id = ?`), id, userID))
}

// ListNotifications returns a user's notifications newest first, capped at
// limit (limit <= 0 returns the full history).
func (s *Store) ListNotifications(ctx context.Context, userID int64, limit int) ([]*Notification, error) {
	q := `SELECT ` + notificationCols + ` FROM notifications WHERE user_id = ?
	      ORDER BY created_at DESC, id DESC`
	args := []any{userID}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.DB.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountUnreadNotifications counts a user's unread notifications -- the topbar
// badge. It runs on every authenticated render, so it is served by
// idx_notifications_user_read rather than touching the rows.
func (s *Store) CountUnreadNotifications(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at IS NULL`),
		userID).Scan(&n)
	return n, err
}

// MarkNotificationRead stamps read_at on one notification, scoped to its owner.
// A row belonging to somebody else, or one already read, matches nothing and is
// a silent no-op -- reporting an error would confirm the row exists.
func (s *Store) MarkNotificationRead(ctx context.Context, id, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE notifications SET read_at = ? WHERE id = ? AND user_id = ? AND read_at IS NULL`),
		nowISO(), id, userID)
	return err
}

// MarkAllNotificationsRead stamps read_at on every unread row a user owns.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE notifications SET read_at = ? WHERE user_id = ? AND read_at IS NULL`),
		nowISO(), userID)
	return err
}

// DeleteNotificationsBefore purges read notifications created before
// readCutoff and unread ones before unreadCutoff, so the table does not grow
// without bound. Unread rows get the longer grace period: an old unread entry
// is one the recipient has still never seen.
func (s *Store) DeleteNotificationsBefore(ctx context.Context, readCutoff, unreadCutoff string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`DELETE FROM notifications
		 WHERE (read_at IS NOT NULL AND created_at < ?)
		    OR (read_at IS NULL AND created_at < ?)`), readCutoff, unreadCutoff)
	return err
}
