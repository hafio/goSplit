package store

import (
	"context"
	"database/sql"
	"errors"
)

// AddFriend creates a bidirectional friendship (idempotent). Both users then
// see each other in their friend lists, even before any shared expense.
func (s *Store) AddFriend(ctx context.Context, ownerID, friendID int64) error {
	if ownerID == friendID {
		return nil
	}
	q := `INSERT INTO friendships (owner_id, friend_id) VALUES (?, ?) ON CONFLICT DO NOTHING`
	if _, err := s.DB.ExecContext(ctx, s.rebind(q), ownerID, friendID); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, s.rebind(q), friendID, ownerID)
	return err
}

// RemoveFriend deletes the friendship in both directions.
func (s *Store) RemoveFriend(ctx context.Context, ownerID, friendID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`DELETE FROM friendships WHERE (owner_id = ? AND friend_id = ?) OR (owner_id = ? AND friend_id = ?)`),
		ownerID, friendID, friendID, ownerID)
	return err
}

// ListFriends returns a user's friends (from explicit friendships), ordered by
// name.
func (s *Store) ListFriends(ctx context.Context, ownerID int64) ([]*User, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT `+prefixCols("u", userCols)+`
		 FROM friendships f JOIN users u ON u.id = f.friend_id
		 WHERE f.owner_id = ? ORDER BY u.name, u.email`), ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AreFriends reports whether an explicit friendship exists.
func (s *Store) AreFriends(ctx context.Context, ownerID, friendID int64) (bool, error) {
	var one int
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT 1 FROM friendships WHERE owner_id = ? AND friend_id = ?`), ownerID, friendID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// prefixCols rewrites a comma-separated column list to prefix each column with
// an alias (e.g. "id, name" -> "u.id, u.name").
func prefixCols(alias, cols string) string {
	var b []byte
	col := make([]byte, 0, 32)
	flush := func() {
		name := trimSpace(string(col))
		if name != "" {
			b = append(b, alias...)
			b = append(b, '.')
			b = append(b, name...)
		}
		col = col[:0]
	}
	depth := 0
	for i := 0; i < len(cols); i++ {
		c := cols[i]
		switch {
		case c == '(':
			depth++
			col = append(col, c)
		case c == ')':
			depth--
			col = append(col, c)
		case c == ',' && depth == 0:
			flush()
			b = append(b, ',', ' ')
		default:
			col = append(col, c)
		}
	}
	flush()
	return string(b)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
