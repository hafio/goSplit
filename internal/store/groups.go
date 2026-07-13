package store

import (
	"context"
	"database/sql"
	"errors"
)

const groupCols = `id, public_id, name, image, created_by, default_currency,
	simplify_debts, archived_at, splitwise_group_id, created_at`

func scanGroup(row interface{ Scan(...any) error }) (*Group, error) {
	var g Group
	err := row.Scan(&g.ID, &g.PublicID, &g.Name, &g.Image, &g.CreatedBy,
		&g.DefaultCurrency, &g.SimplifyDebts, &g.ArchivedAt, &g.SplitwiseGroupID, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateGroup inserts a group and adds the creator as a member.
func (s *Store) CreateGroup(ctx context.Context, g *Group) (*Group, error) {
	if g.PublicID == "" {
		g.PublicID = NewNanoID(12)
	}
	cur := orDefault(g.DefaultCurrency, "USD")
	id, err := s.insertReturningID(ctx,
		`INSERT INTO groups (public_id, name, image, created_by, default_currency, simplify_debts, splitwise_group_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`, "groups",
		g.PublicID, g.Name, g.Image, g.CreatedBy, cur, g.SimplifyDebts, g.SplitwiseGroupID)
	if err != nil {
		return nil, err
	}
	if err := s.AddGroupMember(ctx, id, g.CreatedBy); err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, id)
}

// GetGroup returns a group by id.
func (s *Store) GetGroup(ctx context.Context, id int64) (*Group, error) {
	return scanGroup(s.DB.QueryRowContext(ctx, s.rebind(`SELECT `+groupCols+` FROM groups WHERE id = ?`), id))
}

// GetGroupByPublicID returns a group by its shareable public id.
func (s *Store) GetGroupByPublicID(ctx context.Context, publicID string) (*Group, error) {
	return scanGroup(s.DB.QueryRowContext(ctx, s.rebind(`SELECT `+groupCols+` FROM groups WHERE public_id = ?`), publicID))
}

// GetGroupBySplitwiseID returns a group previously imported from Splitwise, or
// ErrNotFound.
func (s *Store) GetGroupBySplitwiseID(ctx context.Context, splitwiseID string) (*Group, error) {
	return scanGroup(s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT `+groupCols+` FROM groups WHERE splitwise_group_id = ?`), splitwiseID))
}

// AddGroupMember adds a user to a group (idempotent).
func (s *Store) AddGroupMember(ctx context.Context, groupID, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`INSERT INTO group_users (group_id, user_id) VALUES (?, ?) ON CONFLICT DO NOTHING`), groupID, userID)
	return err
}

// RemoveGroupMember removes a user from a group.
func (s *Store) RemoveGroupMember(ctx context.Context, groupID, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`DELETE FROM group_users WHERE group_id = ? AND user_id = ?`), groupID, userID)
	return err
}

// IsGroupMember reports whether a user belongs to a group.
func (s *Store) IsGroupMember(ctx context.Context, groupID, userID int64) (bool, error) {
	var one int
	err := s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT 1 FROM group_users WHERE group_id = ? AND user_id = ?`), groupID, userID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// GroupMembers returns the members of a group ordered by name.
func (s *Store) GroupMembers(ctx context.Context, groupID int64) ([]*User, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT `+prefixCols("u", userCols)+`
		 FROM group_users gu JOIN users u ON u.id = gu.user_id
		 WHERE gu.group_id = ? ORDER BY u.name, u.email`), groupID)
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

// ListGroupsForUser returns the groups a user belongs to. When includeArchived
// is false, archived groups are omitted.
func (s *Store) ListGroupsForUser(ctx context.Context, userID int64, includeArchived bool) ([]*Group, error) {
	q := `SELECT ` + prefixCols("g", groupCols) + `
		FROM group_users gu JOIN groups g ON g.id = gu.group_id
		WHERE gu.user_id = ?`
	if !includeArchived {
		q += ` AND g.archived_at IS NULL`
	}
	q += ` ORDER BY g.name`
	rows, err := s.DB.QueryContext(ctx, s.rebind(q), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SetGroupArchived archives or unarchives a group.
func (s *Store) SetGroupArchived(ctx context.Context, groupID int64, archived bool) error {
	var val any
	if archived {
		val = nowISO()
	}
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE groups SET archived_at = ? WHERE id = ?`), val, groupID)
	return err
}

// SetGroupSimplify toggles debt simplification for a group.
func (s *Store) SetGroupSimplify(ctx context.Context, groupID int64, simplify bool) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE groups SET simplify_debts = ? WHERE id = ?`), simplify, groupID)
	return err
}

// UpdateGroup updates the mutable group fields (name, image, currency).
func (s *Store) UpdateGroup(ctx context.Context, g *Group) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE groups SET name = ?, image = ?, default_currency = ? WHERE id = ?`),
		g.Name, g.Image, g.DefaultCurrency, g.ID)
	return err
}
