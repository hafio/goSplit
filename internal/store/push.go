package store

import "context"

// PushSubscription is a stored Web Push subscription (raw browser JSON).
type PushSubscription struct {
	UserID       int64
	Endpoint     string
	Subscription string // JSON: {endpoint, keys:{p256dh, auth}}
}

// SavePushSubscription stores/updates a subscription for a user+endpoint.
func (s *Store) SavePushSubscription(ctx context.Context, userID int64, endpoint, subscription string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO push_notifications (user_id, endpoint, subscription) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, endpoint) DO UPDATE SET subscription = excluded.subscription`),
		userID, endpoint, subscription)
	return err
}

// DeletePushSubscription removes a subscription by user+endpoint.
func (s *Store) DeletePushSubscription(ctx context.Context, userID int64, endpoint string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`DELETE FROM push_notifications WHERE user_id = ? AND endpoint = ?`), userID, endpoint)
	return err
}

// ListPushSubscriptions returns all subscriptions for the given user ids.
func (s *Store) ListPushSubscriptions(ctx context.Context, userIDs []int64) ([]PushSubscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	// Build an IN (...) clause with `?` placeholders (rebound for Postgres).
	q := `SELECT user_id, endpoint, subscription FROM push_notifications WHERE user_id IN (`
	args := make([]any, 0, len(userIDs))
	for i, id := range userIDs {
		if i > 0 {
			q += ", "
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	rows, err := s.DB.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		var p PushSubscription
		if err := rows.Scan(&p.UserID, &p.Endpoint, &p.Subscription); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
