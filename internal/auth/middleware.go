package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/hafio/gosplit/internal/store"
)

type ctxKey int

const (
	userKey ctxKey = iota
	csrfKey
)

// Cookie names.
const (
	SessionCookie = "gosplit_session"
	CSRFCookie    = "gosplit_csrf"
	// SessionTTL is how long a session lasts.
	SessionTTL = 30 * 24 * time.Hour
)

// Manager wires authentication middleware to the data store.
type Manager struct {
	Store  *store.Store
	Secure bool // Secure cookie flag (true under https)
}

// NewManager constructs an auth Manager.
func NewManager(s *store.Store, secure bool) *Manager {
	return &Manager{Store: s, Secure: secure}
}

// UserFrom returns the authenticated user from the request context, or nil.
func UserFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userKey).(*store.User)
	return u
}

// CSRFFrom returns the CSRF token for the request, or "".
func CSRFFrom(ctx context.Context) string {
	t, _ := ctx.Value(csrfKey).(string)
	return t
}

// Authenticate loads the session user (if any) into the request context and
// ensures a CSRF token cookie exists. It never rejects; downstream RequireUser
// enforces access.
func (m *Manager) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
			if sess, err := m.Store.GetSession(ctx, c.Value); err == nil {
				if u, err := m.Store.GetUser(ctx, sess.UserID); err == nil && u.IsActive() {
					ctx = context.WithValue(ctx, userKey, u)
				}
			}
		}

		// Ensure a CSRF token exists (double-submit cookie pattern).
		token := ""
		if c, err := r.Cookie(CSRFCookie); err == nil && c.Value != "" {
			token = c.Value
		} else {
			token = RandomToken(32)
			http.SetCookie(w, &http.Cookie{
				Name:     CSRFCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: false, // readable so SPA/htmx could echo it; forms use the hidden field
				Secure:   m.Secure,
				SameSite: http.SameSiteLaxMode,
			})
		}
		ctx = context.WithValue(ctx, csrfKey, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VerifyCSRF rejects mutating requests whose form token does not match the CSRF
// cookie. Safe methods (GET/HEAD/OPTIONS) pass through.
func (m *Manager) VerifyCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(CSRFCookie)
		if err != nil || cookie.Value == "" {
			http.Error(w, "missing CSRF token", http.StatusForbidden)
			return
		}
		form := r.Header.Get("X-CSRF-Token")
		if form == "" {
			form = r.FormValue("csrf_token")
		}
		if subtle.ConstantTimeCompare([]byte(form), []byte(cookie.Value)) != 1 {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUser redirects unauthenticated requests to the login page.
func (m *Manager) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 for non-admins (and redirects the logged-out).
func (m *Manager) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !u.IsAdmin() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetSession creates a session row and writes the session cookie.
func (m *Manager) SetSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token := RandomToken(32)
	if _, err := m.Store.CreateSession(r.Context(), token, userID, SessionTTL); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(SessionTTL),
	})
	return nil
}

// ClearSession deletes the session row and expires the cookie (logout).
func (m *Manager) ClearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		_ = m.Store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
