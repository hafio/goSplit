package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/store"
)

// Admin-management errors surfaced to handlers.
var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrInvalidRole      = errors.New("role must be USER or ADMIN")
	ErrEmailRequired    = errors.New("email is required")
)

// AdminCreateUser creates a user on behalf of an admin. Email is required and
// must be unique; role must be USER/ADMIN (defaults USER); currency/language
// fall back to store defaults when blank. A blank password leaves the account
// password-less (magic-link sign-in); a set password must be ≥ 8 chars.
func (s *Service) AdminCreateUser(ctx context.Context, name, email, role, currency, lang, password string) (*store.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrEmailRequired
	}
	if role == "" {
		role = "USER"
	}
	if role != "USER" && role != "ADMIN" {
		return nil, ErrInvalidRole
	}
	if _, err := s.Store.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	u := &store.User{
		Name:              strings.TrimSpace(name),
		Email:             email,
		Role:              role,
		Currency:          strings.TrimSpace(currency),
		PreferredLanguage: strings.TrimSpace(lang),
	}
	if password != "" {
		if len(password) < 8 {
			return nil, ErrPasswordTooShort
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = sql.NullString{String: hash, Valid: true}
	}
	return s.Store.CreateUser(ctx, u)
}

// AdminSetPassword sets a user's password and revokes all of their existing
// sessions (logs them out everywhere).
func (s *Service) AdminSetPassword(ctx context.Context, userID int64, password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.Store.SetPassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.Store.DeleteUserSessions(ctx, userID)
}
