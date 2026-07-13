package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("store: not found")

const userCols = `id, name, email, email_verified, password_hash, image, currency,
	default_currency, preferred_language, theme_color, role, deactivated_at, banking_id,
	hidden_friend_ids, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var hidden string
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.EmailVerified, &u.PasswordHash,
		&u.Image, &u.Currency, &u.DefaultCurrency, &u.PreferredLanguage, &u.ThemeColor, &u.Role,
		&u.DeactivatedAt, &u.BankingID, &hidden, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if hidden != "" {
		_ = json.Unmarshal([]byte(hidden), &u.HiddenFriendIDs)
	}
	return &u, nil
}

// GetUser returns a user by id.
func (s *Store) GetUser(ctx context.Context, id int64) (*User, error) {
	row := s.DB.QueryRowContext(ctx, s.rebind(`SELECT `+userCols+` FROM users WHERE id = ?`), id)
	return scanUser(row)
}

// GetUserByEmail returns a user by (lower-cased) email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.DB.QueryRowContext(ctx, s.rebind(`SELECT `+userCols+` FROM users WHERE email = ?`),
		strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

// CreateUser inserts a user and returns it with the assigned id. Email is
// normalized to lower-case. hidden defaults to an empty JSON array.
func (s *Store) CreateUser(ctx context.Context, u *User) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(u.Email))
	hidden, _ := json.Marshal(u.HiddenFriendIDs)
	if len(u.HiddenFriendIDs) == 0 {
		hidden = []byte("[]")
	}
	role := u.Role
	if role == "" {
		role = "USER"
	}
	cur := u.Currency
	if cur == "" {
		cur = "USD"
	}
	lang := u.PreferredLanguage
	if lang == "" {
		lang = "en"
	}
	theme := u.ThemeColor
	if theme == "" {
		theme = "burgundy"
	}
	q := `INSERT INTO users (name, email, email_verified, password_hash, image,
		currency, default_currency, preferred_language, theme_color, role, hidden_friend_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	id, err := s.insertReturningID(ctx, q, "users",
		u.Name, email, u.EmailVerified, u.PasswordHash, u.Image,
		cur, orDefault(u.DefaultCurrency, cur), lang, theme, role, string(hidden))
	if err != nil {
		return nil, err
	}
	return s.GetUser(ctx, id)
}

// SetPassword updates a user's argon2id password hash.
func (s *Store) SetPassword(ctx context.Context, userID int64, hash string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE users SET password_hash = ? WHERE id = ?`), hash, userID)
	return err
}

// UpdateProfile updates editable profile fields.
func (s *Store) UpdateProfile(ctx context.Context, u *User) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE users SET name = ?, currency = ?, default_currency = ?, preferred_language = ?, theme_color = ?, image = ? WHERE id = ?`),
		u.Name, u.Currency, u.DefaultCurrency, u.PreferredLanguage, u.ThemeColor, u.Image, u.ID)
	return err
}

// AdminUpdateUser updates the admin-editable identity fields of any user
// (name, email, role, currency, language). Email is normalized to lower-case;
// a duplicate email surfaces the UNIQUE-constraint error to the caller.
func (s *Store) AdminUpdateUser(ctx context.Context, u *User) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE users SET name = ?, email = ?, role = ?, currency = ?, preferred_language = ? WHERE id = ?`),
		u.Name, strings.ToLower(strings.TrimSpace(u.Email)), u.Role, u.Currency, u.PreferredLanguage, u.ID)
	return err
}

// SetRole sets a user's role (used for admin auto-promotion; never auto-demote).
func (s *Store) SetRole(ctx context.Context, userID int64, role string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE users SET role = ? WHERE id = ?`), role, userID)
	return err
}

// MarkEmailVerified records that a user's email is verified (idempotent).
func (s *Store) MarkEmailVerified(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE users SET email_verified = ? WHERE id = ? AND email_verified IS NULL`),
		nowISO(), userID)
	return err
}

// SetDeactivated activates or deactivates an account.
func (s *Store) SetDeactivated(ctx context.Context, userID int64, deactivated bool) error {
	var val any
	if deactivated {
		val = nowISO()
	} else {
		val = nil
	}
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE users SET deactivated_at = ? WHERE id = ?`), val, userID)
	return err
}

// SetHiddenFriends persists the hidden friend id list as JSON text.
func (s *Store) SetHiddenFriends(ctx context.Context, userID int64, ids []int64) error {
	if ids == nil {
		ids = []int64{}
	}
	b, _ := json.Marshal(ids)
	_, err := s.DB.ExecContext(ctx, s.rebind(`UPDATE users SET hidden_friend_ids = ? WHERE id = ?`), string(b), userID)
	return err
}

// ListUsers returns all users ordered by id (admin panel).
func (s *Store) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY id`)
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

// DeleteUser removes a user (cascades sessions, memberships, friendships).
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(`DELETE FROM users WHERE id = ?`), userID)
	return err
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
