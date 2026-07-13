package store

import "context"

// CumulatedBalance is a per-friend, per-currency net across all groups + direct.
type CumulatedBalance struct {
	FriendID int64
	Currency string
	Amount   int64 // >0 friend owes you; <0 you owe friend
}

// scanBalances reads balance_view rows.
func scanBalances(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]Balance, error) {
	defer rows.Close()
	var out []Balance
	for rows.Next() {
		var b Balance
		if err := rows.Scan(&b.UserID, &b.FriendID, &b.GroupID, &b.Currency, &b.Amount); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CumulatedBalances returns each friend's net per currency for a user, summed
// across all groups and direct balances (per-currency, never cross-summed).
func (s *Store) CumulatedBalances(ctx context.Context, userID int64) ([]CumulatedBalance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT friend_id, currency, SUM(amount) AS amount
		 FROM balance_view WHERE user_id = ?
		 GROUP BY friend_id, currency
		 HAVING SUM(amount) <> 0
		 ORDER BY friend_id, currency`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CumulatedBalance
	for rows.Next() {
		var c CumulatedBalance
		if err := rows.Scan(&c.FriendID, &c.Currency, &c.Amount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FriendBalance returns the per-currency net between a user and one friend
// (direct + all shared groups combined).
func (s *Store) FriendBalance(ctx context.Context, userID, friendID int64) ([]CumulatedBalance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT friend_id, currency, SUM(amount) AS amount
		 FROM balance_view WHERE user_id = ? AND friend_id = ?
		 GROUP BY friend_id, currency
		 HAVING SUM(amount) <> 0
		 ORDER BY currency`), userID, friendID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CumulatedBalance
	for rows.Next() {
		var c CumulatedBalance
		if err := rows.Scan(&c.FriendID, &c.Currency, &c.Amount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GroupBalances returns the raw per-pair balances within a group.
func (s *Store) GroupBalances(ctx context.Context, groupID int64) ([]Balance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT user_id, friend_id, group_id, currency, amount
		 FROM balance_view WHERE group_id = ? AND amount <> 0
		 ORDER BY currency, user_id, friend_id`), groupID)
	if err != nil {
		return nil, err
	}
	return scanBalances(rows)
}

// UserGroupBalances returns one user's balances vs each other member of a group.
func (s *Store) UserGroupBalances(ctx context.Context, groupID, userID int64) ([]Balance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT user_id, friend_id, group_id, currency, amount
		 FROM balance_view WHERE group_id = ? AND user_id = ? AND amount <> 0
		 ORDER BY currency, friend_id`), groupID, userID)
	if err != nil {
		return nil, err
	}
	return scanBalances(rows)
}
