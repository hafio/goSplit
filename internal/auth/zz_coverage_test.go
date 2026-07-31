package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// covOpenStore opens a fresh SQLite store backed by a temp file (migrations run
// on open). Mirrors store.openTestStore, which is unexported to package store.
func covOpenStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	st, err := store.Open(context.Background(), &config.Config{
		DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// covNewUser inserts a user with the given role and returns it.
func covNewUser(t *testing.T, st *store.Store, name, email, role string) *store.User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), &store.User{Name: name, Email: email, Role: role})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// covOKHandler returns a handler that records invocation and writes 200.
func covOKHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCovNewManager(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, true)
	if m.Store != st {
		t.Fatal("Store not wired")
	}
	if !m.Secure {
		t.Fatal("Secure flag not set")
	}
}

func TestCovUserFrom(t *testing.T) {
	if u := UserFrom(context.Background()); u != nil {
		t.Fatalf("empty context should yield nil, got %v", u)
	}
	want := &store.User{ID: 7, Name: "Zoe"}
	ctx := context.WithValue(context.Background(), userKey, want)
	if got := UserFrom(ctx); got != want {
		t.Fatalf("UserFrom = %v, want %v", got, want)
	}
}

func TestCovCSRFFrom(t *testing.T) {
	if s := CSRFFrom(context.Background()); s != "" {
		t.Fatalf("empty context should yield \"\", got %q", s)
	}
	ctx := context.WithValue(context.Background(), csrfKey, "tok123")
	if got := CSRFFrom(ctx); got != "tok123" {
		t.Fatalf("CSRFFrom = %q, want tok123", got)
	}
}

func TestCovAuthenticate_NoCookies_SetsCSRF(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, true)

	var gotUser *store.User
	var gotCSRF string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFrom(r.Context())
		gotCSRF = CSRFFrom(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	m.Authenticate(inner).ServeHTTP(rec, req)

	if gotUser != nil {
		t.Fatalf("no session -> user should be nil, got %v", gotUser)
	}
	if gotCSRF == "" {
		t.Fatal("CSRF token should be generated and placed in context")
	}
	var csrfCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == CSRFCookie {
			csrfCookie = c
		}
	}
	if csrfCookie == nil {
		t.Fatal("CSRF cookie should be set when absent")
	}
	if csrfCookie.Value != gotCSRF {
		t.Fatalf("cookie value %q != context token %q", csrfCookie.Value, gotCSRF)
	}
	if !csrfCookie.Secure {
		t.Fatal("Secure manager should set Secure cookie")
	}
}

func TestCovAuthenticate_ExistingCSRFReused(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, false)

	var gotCSRF string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCSRF = CSRFFrom(r.Context())
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "preexisting"})
	m.Authenticate(inner).ServeHTTP(rec, req)

	if gotCSRF != "preexisting" {
		t.Fatalf("existing CSRF should be reused, got %q", gotCSRF)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == CSRFCookie {
			t.Fatal("no new CSRF cookie should be set when one exists")
		}
	}
}

func TestCovAuthenticate_ValidSessionLoadsUser(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, false)
	ctx := context.Background()
	u := covNewUser(t, st, "Alice", "alice@example.com", "USER")
	token := RandomToken(32)
	if _, err := st.CreateSession(ctx, token, u.ID, SessionTTL); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var gotUser *store.User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFrom(r.Context())
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	m.Authenticate(inner).ServeHTTP(rec, req)

	if gotUser == nil || gotUser.ID != u.ID {
		t.Fatalf("valid session should load user %d, got %v", u.ID, gotUser)
	}
}

func TestCovAuthenticate_UnknownSessionNoUser(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, false)

	var gotUser *store.User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFrom(r.Context())
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: "no-such-token"})
	m.Authenticate(inner).ServeHTTP(rec, req)

	if gotUser != nil {
		t.Fatalf("unknown session -> user nil, got %v", gotUser)
	}
}

func TestCovAuthenticate_InactiveUserNotLoaded(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, false)
	ctx := context.Background()
	u := covNewUser(t, st, "Bob", "bob@example.com", "USER")
	if err := st.SetDeactivated(ctx, u.ID, true); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	token := RandomToken(32)
	if _, err := st.CreateSession(ctx, token, u.ID, SessionTTL); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var gotUser *store.User
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFrom(r.Context())
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	m.Authenticate(inner).ServeHTTP(rec, req)

	if gotUser != nil {
		t.Fatalf("inactive user should not be loaded, got %v", gotUser)
	}
}

func TestCovVerifyCSRF_SafeMethodPasses(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		called := false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/", nil)
		m.VerifyCSRF(covOKHandler(&called)).ServeHTTP(rec, req)
		if !called {
			t.Fatalf("%s should pass through CSRF guard", method)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: code = %d, want 200", method, rec.Code)
		}
	}
}

func TestCovVerifyCSRF_MissingCookie(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	m.VerifyCSRF(covOKHandler(&called)).ServeHTTP(rec, req)
	if called {
		t.Fatal("handler should not run without CSRF cookie")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", rec.Code)
	}
}

func TestCovVerifyCSRF_Mismatch(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("csrf_token=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "right"})
	m.VerifyCSRF(covOKHandler(&called)).ServeHTTP(rec, req)
	if called {
		t.Fatal("handler should not run on token mismatch")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", rec.Code)
	}
}

func TestCovVerifyCSRF_HeaderMatch(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "match")
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "match"})
	m.VerifyCSRF(covOKHandler(&called)).ServeHTTP(rec, req)
	if !called {
		t.Fatal("matching X-CSRF-Token header should pass")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestCovVerifyCSRF_FormFieldMatch(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("csrf_token=formtok"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "formtok"})
	m.VerifyCSRF(covOKHandler(&called)).ServeHTTP(rec, req)
	if !called {
		t.Fatal("matching csrf_token form field should pass")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestCovRequireUser(t *testing.T) {
	m := NewManager(covOpenStore(t), false)

	// Unauthenticated -> redirect to login with next.
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	m.RequireUser(covOKHandler(&called)).ServeHTTP(rec, req)
	if called {
		t.Fatal("unauth request should not reach handler")
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?next=/dashboard" {
		t.Fatalf("Location = %q, want /login?next=/dashboard", loc)
	}

	// Authenticated -> passes.
	called = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, &store.User{ID: 1}))
	m.RequireUser(covOKHandler(&called)).ServeHTTP(rec, req)
	if !called {
		t.Fatal("authenticated request should reach handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestCovRequireAdmin(t *testing.T) {
	m := NewManager(covOpenStore(t), false)

	// Logged out -> redirect to /login.
	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	m.RequireAdmin(covOKHandler(&called)).ServeHTTP(rec, req)
	if called {
		t.Fatal("logged-out request should not reach handler")
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}

	// Non-admin -> 403.
	called = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, &store.User{ID: 2, Role: "USER"}))
	m.RequireAdmin(covOKHandler(&called)).ServeHTTP(rec, req)
	if called {
		t.Fatal("non-admin should not reach handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", rec.Code)
	}

	// Admin -> passes.
	called = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, &store.User{ID: 3, Role: "ADMIN"}))
	m.RequireAdmin(covOKHandler(&called)).ServeHTTP(rec, req)
	if !called {
		t.Fatal("admin should reach handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestCovSetSession(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, true)
	ctx := context.Background()
	u := covNewUser(t, st, "Carol", "carol@example.com", "USER")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := m.SetSession(rec, req, u.ID); err != nil {
		t.Fatalf("SetSession: %v", err)
	}

	var sc *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			sc = c
		}
	}
	if sc == nil {
		t.Fatal("session cookie should be set")
	}
	if sc.Value == "" {
		t.Fatal("session cookie should carry a token")
	}
	if !sc.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if !sc.Secure {
		t.Fatal("secure manager must set Secure on session cookie")
	}
	// The token should resolve to a live session for the user.
	sess, err := st.GetSession(ctx, sc.Value)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.UserID != u.ID {
		t.Fatalf("session UserID = %d, want %d", sess.UserID, u.ID)
	}
}

func TestCovClearSession(t *testing.T) {
	st := covOpenStore(t)
	m := NewManager(st, false)
	ctx := context.Background()
	u := covNewUser(t, st, "Dave", "dave@example.com", "USER")
	token := RandomToken(32)
	if _, err := st.CreateSession(ctx, token, u.ID, SessionTTL); err != nil {
		t.Fatalf("create session: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	m.ClearSession(rec, req)

	// Session row deleted.
	if _, err := st.GetSession(ctx, token); err == nil {
		t.Fatal("session should be deleted after ClearSession")
	}
	// Cookie expired (MaxAge < 0).
	var sc *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			sc = c
		}
	}
	if sc == nil {
		t.Fatal("expired session cookie should be written")
	}
	if sc.MaxAge >= 0 {
		t.Fatalf("MaxAge = %d, want negative (expired)", sc.MaxAge)
	}
}

func TestCovClearSession_NoCookieNoPanic(t *testing.T) {
	m := NewManager(covOpenStore(t), false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	// No session cookie present: should still write the expiring cookie.
	m.ClearSession(rec, req)
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("ClearSession should expire the cookie even without an incoming one")
	}
}
