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
// across all direct balances and non-archived groups (per-currency, never
// cross-summed). Archived groups leave the aggregate totals; their own debt is
// still visible per-group via UserGroupNets/GroupBalances.
func (s *Store) CumulatedBalances(ctx context.Context, userID int64) ([]CumulatedBalance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT friend_id, currency, SUM(amount) AS amount
		 FROM balance_view WHERE user_id = ?
		   AND (group_id IS NULL OR group_id NOT IN (SELECT id FROM groups WHERE archived_at IS NOT NULL))
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
// (direct + all shared non-archived groups combined; archived groups excluded).
func (s *Store) FriendBalance(ctx context.Context, userID, friendID int64) ([]CumulatedBalance, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT friend_id, currency, SUM(amount) AS amount
		 FROM balance_view WHERE user_id = ? AND friend_id = ?
		   AND (group_id IS NULL OR group_id NOT IN (SELECT id FROM groups WHERE archived_at IS NOT NULL))
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

// GroupMemberCounts returns the member count of every group the user belongs to.
func (s *Store) GroupMemberCounts(ctx context.Context, userID int64) (map[int64]int, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT group_id, COUNT(*) FROM group_users
		 WHERE group_id IN (SELECT group_id FROM group_users WHERE user_id = ?)
		 GROUP BY group_id`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var gid int64
		var n int
		if err := rows.Scan(&gid, &n); err != nil {
			return nil, err
		}
		out[gid] = n
	}
	return out, rows.Err()
}

// UserGroupNets returns the user's net position per group per currency in one
// pass over balance_view (groups list page).
func (s *Store) UserGroupNets(ctx context.Context, userID int64) (map[int64]map[string]int64, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT group_id, currency, SUM(amount) FROM balance_view
		 WHERE user_id = ? AND group_id IS NOT NULL
		 GROUP BY group_id, currency`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]int64{}
	for rows.Next() {
		var gid int64
		var cur string
		var amt int64
		if err := rows.Scan(&gid, &cur, &amt); err != nil {
			return nil, err
		}
		if out[gid] == nil {
			out[gid] = map[string]int64{}
		}
		out[gid][cur] += amt
	}
	return out, rows.Err()
}
