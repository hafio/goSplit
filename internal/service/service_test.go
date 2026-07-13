package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// fakeMailer captures sent messages for assertions.
type fakeMailer struct{ msgs []struct{ To, Subject, Body string } }

func (m *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	m.msgs = append(m.msgs, struct{ To, Subject, Body string }{to, subject, body})
	return nil
}
func (m *fakeMailer) last() string {
	if len(m.msgs) == 0 {
		return ""
	}
	return m.msgs[len(m.msgs)-1].Body
}

func newTestService(t *testing.T) (*Service, *fakeMailer) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "svc.db")
	st, err := store.Open(context.Background(), &config.Config{DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := &config.Config{
		BaseURL: "http://test.local", EnableSendingInvites: true,
		DefaultHomepage: "/balances", AdminEmails: []string{"admin@example.com"},
	}
	m := &fakeMailer{}
	svc := New(st, m, cfg)
	svc.SetSynchronousEmail() // deterministic mail assertions in tests
	return svc, m
}

func TestRegisterAndLogin(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	u, err := svc.Register(ctx, "Alice", "Alice@Example.com", "hunter2pass")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "alice@example.com" {
		t.Fatalf("email not normalized: %q", u.Email)
	}
	// Duplicate registration fails.
	if _, err := svc.Register(ctx, "Alice2", "alice@example.com", "other"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
	// Correct login.
	if _, err := svc.LoginPassword(ctx, "alice@example.com", "hunter2pass"); err != nil {
		t.Fatalf("login should succeed: %v", err)
	}
	// Wrong password.
	if _, err := svc.LoginPassword(ctx, "alice@example.com", "nope"); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("want ErrInvalidLogin, got %v", err)
	}
	// Unknown user (uniform error).
	if _, err := svc.LoginPassword(ctx, "ghost@example.com", "x"); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("want ErrInvalidLogin for unknown, got %v", err)
	}
}

func TestAdminAutoPromotion(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u, err := svc.Register(ctx, "Admin", "admin@example.com", "hunter2pass")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "ADMIN" {
		t.Fatalf("admin email should be promoted, role=%s", u.Role)
	}
}

func TestMagicLinkFlow(t *testing.T) {
	svc, mailer := newTestService(t)
	ctx := context.Background()
	if err := svc.RequestMagicLink(ctx, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	body := mailer.last()
	token := extractToken(body, "token=")
	if token == "" {
		t.Fatalf("no token in magic-link email: %q", body)
	}
	u, err := svc.ConsumeMagicLink(ctx, token)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if u.Email != "bob@example.com" {
		t.Fatalf("wrong user: %q", u.Email)
	}
	// Token is single-use.
	if _, err := svc.ConsumeMagicLink(ctx, token); err == nil {
		t.Fatal("magic token should be single-use")
	}
}

func TestForgotResetPassword(t *testing.T) {
	svc, mailer := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "Carol", "carol@example.com", "oldpassword"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ForgotPassword(ctx, "carol@example.com"); err != nil {
		t.Fatal(err)
	}
	token := extractToken(mailer.last(), "token=")
	if token == "" {
		t.Fatal("no reset token emailed")
	}
	if err := svc.ResetPassword(ctx, token, "newpassword1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := svc.LoginPassword(ctx, "carol@example.com", "newpassword1"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	// Forgot for unknown email is a silent success (no enumeration).
	if err := svc.ForgotPassword(ctx, "nobody@example.com"); err != nil {
		t.Fatalf("forgot unknown should succeed silently: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u, _ := svc.Register(ctx, "Dave", "dave@example.com", "firstpass1")
	if err := svc.ChangePassword(ctx, u.ID, "wrong", "secondpass1"); err == nil {
		t.Fatal("change with wrong current should fail")
	}
	if err := svc.ChangePassword(ctx, u.ID, "firstpass1", "secondpass1"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if _, err := svc.LoginPassword(ctx, "dave@example.com", "secondpass1"); err != nil {
		t.Fatalf("login after change: %v", err)
	}
}

// fakeRates implements currency.Provider for conversion tests.
type fakeRates struct{ rate string }

func (f fakeRates) Name() string { return "fake" }
func (f fakeRates) Rate(context.Context, string, string, string) (string, error) {
	return f.rate, nil
}

func TestCreateConversion(t *testing.T) {
	svc, _ := newTestService(t)
	svc.Rates = fakeRates{rate: "0.9"}
	ctx := context.Background()
	a, _ := svc.Register(ctx, "A", "a@example.com", "password12")
	b, _ := svc.Register(ctx, "B", "b@example.com", "password12")

	e, err := svc.CreateConversion(ctx, a.ID, a.ID, b.ID, 10000, "USD", "EUR", "2025-01-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !e.ConversionToID.Valid {
		t.Fatal("from-expense should link to the to-expense")
	}
	// The linked EUR leg should exist with 9000 minor units.
	to, err := svc.Store.GetExpense(ctx, e.ConversionToID.String)
	if err != nil {
		t.Fatal(err)
	}
	if to.Amount != 9000 || to.Currency != "EUR" {
		t.Fatalf("to-leg amount/currency = %d/%s, want 9000/EUR", to.Amount, to.Currency)
	}
	// Rate should now be cached.
	if r, err := svc.Store.GetCachedRate(ctx, "USD", "EUR", "2025-01-01"); err != nil || r != "0.9" {
		t.Fatalf("rate not cached: %q %v", r, err)
	}
}

func TestImportFromSplitwise(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	actor, _ := svc.Register(ctx, "Owner", "owner@example.com", "password12")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/get_friends":
			_, _ = w.Write([]byte(`{"friends":[{"email":"friend@example.com","first_name":"Fr","last_name":"Iend"}]}`))
		case "/get_groups":
			_, _ = w.Write([]byte(`{"groups":[{"id":42,"name":"Trip","members":[{"email":"member@example.com","first_name":"Mem"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	orig := splitwiseBase
	splitwiseBase = srv.URL
	defer func() { splitwiseBase = orig }()

	res, err := svc.ImportFromSplitwise(ctx, actor, "testkey")
	if err != nil {
		t.Fatal(err)
	}
	if res.Friends != 1 || res.Groups != 1 {
		t.Fatalf("import result = %+v, want 1 friend/1 group", res)
	}
	// Re-import is idempotent for the group (matched by splitwise id).
	res2, err := svc.ImportFromSplitwise(ctx, actor, "testkey")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Groups != 0 {
		t.Fatalf("re-import created %d groups, want 0 (idempotent)", res2.Groups)
	}
}

// extractToken pulls the token value following marker from a message body.
func extractToken(body, marker string) string {
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	end := strings.IndexAny(rest, "\r\n \t")
	if end < 0 {
		return rest
	}
	return rest[:end]
}
