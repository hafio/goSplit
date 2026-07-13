package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/store"
)

// Auth-related errors surfaced to handlers.
var (
	ErrEmailTaken       = errors.New("an account with that email already exists")
	ErrInvalidLogin     = errors.New("invalid email or password")
	ErrSignupDisabled   = errors.New("email signup is disabled")
	ErrNoPassword       = errors.New("this account has no password set; use the magic link or reset flow")
	ErrAccountInactive  = errors.New("this account has been deactivated")
)

const (
	magicTTL  = 15 * time.Minute
	resetTTL  = 1 * time.Hour
)

// Register creates a password account, applies admin auto-promotion, and
// returns the new user. Honors DISABLE_EMAIL_SIGNUP.
func (s *Service) Register(ctx context.Context, name, email, password string) (*store.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.Config.DisableEmailSignup {
		return nil, ErrSignupDisabled
	}
	if _, err := s.Store.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	role := "USER"
	if s.Config.IsAdminEmail(email) {
		role = "ADMIN"
	}
	u, err := s.Store.CreateUser(ctx, &store.User{
		Name:         name,
		Email:        email,
		PasswordHash: sql.NullString{String: hash, Valid: true},
		Role:         role,
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// LoginPassword authenticates an email + password. Blocks deactivated accounts.
func (s *Service) LoginPassword(ctx context.Context, email, password string) (*store.User, error) {
	u, err := s.Store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, store.ErrNotFound) {
		// Uniform error to avoid user enumeration; still do a dummy hash compare.
		_ = auth.VerifyPassword(password, "$argon2id$v=19$m=65536,t=1,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		return nil, ErrInvalidLogin
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive() {
		return nil, ErrAccountInactive
	}
	if !u.PasswordHash.Valid {
		return nil, ErrNoPassword
	}
	if err := auth.VerifyPassword(password, u.PasswordHash.String); err != nil {
		return nil, ErrInvalidLogin
	}
	s.promoteAdminIfNeeded(ctx, u)
	return u, nil
}

// RequestMagicLink issues a magic-link token and emails it. Silently succeeds
// for unknown emails when signup is disabled (no enumeration); otherwise it
// creates a pending user so first-time magic-link sign-in works.
func (s *Service) RequestMagicLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email is required")
	}
	_, err := s.Store.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		if s.Config.DisableEmailSignup {
			return nil // pretend success
		}
		role := "USER"
		if s.Config.IsAdminEmail(email) {
			role = "ADMIN"
		}
		if _, err := s.Store.CreateUser(ctx, &store.User{Email: email, Role: role}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	token := auth.RandomToken(32)
	if err := s.Store.CreateVerificationToken(ctx, email, token, "magic", magicTTL); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/auth/magic?token=%s", s.Config.BaseURL, token)
	// Send off the request path so a slow/unreachable SMTP server can't hang the
	// sign-in form; the token is already persisted.
	s.sendMailAsync(func() {
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Mail.Send(sctx, email, "Your GoSplit sign-in link",
			"Click to sign in (valid 15 minutes):\n\n"+link+"\n\nIf you didn't request this, ignore this email."); err != nil {
			slog.Warn("magic-link email failed", "to", email, "err", err)
		}
	})
	return nil
}

// AdminMagicLinkForUser mints a fresh magic-link URL for a user and returns it
// directly (no email) so an admin can hand it to someone who can't receive mail.
// Same token type and TTL as the normal sign-in link.
func (s *Service) AdminMagicLinkForUser(ctx context.Context, userID int64) (string, error) {
	u, err := s.Store.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	token := auth.RandomToken(32)
	if err := s.Store.CreateVerificationToken(ctx, u.Email, token, "magic", magicTTL); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/auth/magic?token=%s", s.Config.BaseURL, token), nil
}

// ConsumeMagicLink validates a magic-link token and returns the user, marking
// the email verified and applying admin auto-promotion.
func (s *Service) ConsumeMagicLink(ctx context.Context, token string) (*store.User, error) {
	email, err := s.Store.UseVerificationToken(ctx, token, "magic")
	if err != nil {
		return nil, errors.New("this sign-in link is invalid or has expired")
	}
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if !u.IsActive() {
		return nil, ErrAccountInactive
	}
	_ = s.Store.MarkEmailVerified(ctx, u.ID)
	s.promoteAdminIfNeeded(ctx, u)
	return u, nil
}

// ChangePassword updates an authenticated user's password after verifying the
// current one (or sets one if none existed).
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	u, err := s.Store.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if u.PasswordHash.Valid {
		if err := auth.VerifyPassword(current, u.PasswordHash.String); err != nil {
			return errors.New("current password is incorrect")
		}
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.Store.SetPassword(ctx, userID, hash); err != nil {
		return err
	}
	// Rotate sessions on password change.
	return s.Store.DeleteUserSessions(ctx, userID)
}

// ForgotPassword issues a reset token and emails it (no enumeration).
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.Store.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil // pretend success
	}
	if err != nil {
		return err
	}
	token := auth.RandomToken(32)
	if err := s.Store.CreateVerificationToken(ctx, email, token, "reset", resetTTL); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", s.Config.BaseURL, token)
	_ = u
	return s.Mail.Send(ctx, email, "Reset your GoSplit password",
		"Click to reset your password (valid 1 hour):\n\n"+link+"\n\nIf you didn't request this, ignore this email.")
}

// ResetPassword consumes a reset token and sets a new password.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	email, err := s.Store.UseVerificationToken(ctx, token, "reset")
	if err != nil {
		return errors.New("this reset link is invalid or has expired")
	}
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Store.SetPassword(ctx, u.ID, hash); err != nil {
		return err
	}
	_ = s.Store.MarkEmailVerified(ctx, u.ID)
	return s.Store.DeleteUserSessions(ctx, u.ID)
}

// promoteAdminIfNeeded upgrades a user to ADMIN if their email is in
// ADMIN_EMAILS (never auto-demotes).
func (s *Service) promoteAdminIfNeeded(ctx context.Context, u *store.User) {
	if u.Role != "ADMIN" && s.Config.IsAdminEmail(u.Email) {
		if err := s.Store.SetRole(ctx, u.ID, "ADMIN"); err == nil {
			u.Role = "ADMIN"
		}
	}
}
