package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/push"
	"github.com/hafio/gosplit/internal/store"
)

// covFailMailer is a mail.Mailer whose Send always fails, used to cover the
// email-failure log branches that the always-succeeding fakeMailer cannot.
type covFailMailer struct{}

func (covFailMailer) Send(context.Context, string, string, string) error {
	return errors.New("smtp down")
}

// TestCovDispatchDefaults covers the two goroutine-spawning dispatch paths in
// service.go that SetSynchronousEmail hides in the other tests: the default
// New() dispatch closure ("go f()") and sendMailAsync's nil-dispatch fallback.
// Each waits on a channel the spawned func closes, so there is no timing
// flakiness -- the timeout only guards against a total failure to run.
func TestCovDispatchDefaults(t *testing.T) {
	svc, _ := newTestService(t)
	// A fresh Service via New keeps the default goroutine dispatch closure
	// (newTestService replaced svc's with the synchronous one).
	svc2 := New(svc.Store, svc.Mail, svc.Config)

	done := make(chan struct{})
	svc2.sendMailAsync(func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("default dispatch never ran f")
	}

	// nil dispatch -> sendMailAsync spawns its own goroutine.
	svc2.dispatch = nil
	done2 := make(chan struct{})
	svc2.sendMailAsync(func() { close(done2) })
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("nil-dispatch fallback never ran f")
	}
}

// TestCovPushEnabledSendError covers pushToUsers past the enabled gate: with an
// enabled Sender and no subscriptions it lists + builds the payload over an
// empty loop, and with one stored but invalid-JSON subscription the Send call
// fails inside push (before any network I/O), exercising the send-error branch.
func TestCovPushEnabledSendError(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")

	// Non-empty VAPID keys make the Sender report Enabled(); delivery of an
	// invalid subscription fails at json.Unmarshal, never reaching the network.
	svc.Config.WebPushPublicKey = "pub"
	svc.Config.WebPushPrivateKey = "priv"
	svc.Push = push.New(svc.Config)
	if !svc.Push.Enabled() {
		t.Fatal("push should be enabled with VAPID keys set")
	}
	e := &store.Expense{ID: "exp-1", Name: "Dinner", Amount: 1000, Currency: "USD"}

	// Enabled, no subscriptions: covers ListPushSubscriptions + payload build.
	svc.pushToUsers(ctx, []int64{u.ID}, "title", e)

	// One invalid-JSON subscription: Send returns an error (no network I/O),
	// covering the "else if err != nil" warn branch of the delivery loop.
	if err := svc.Store.SavePushSubscription(ctx, u.ID, "https://push.example/ep", "not-json"); err != nil {
		t.Fatalf("save subscription: %v", err)
	}
	svc.pushToUsers(ctx, []int64{u.ID}, "title", e)
}

// TestCovEmailParticipantsSendError covers emailParticipants' mail-failure
// branch by swapping in a mailer whose Send always errors.
func TestCovEmailParticipantsSendError(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")
	svc.Mail = covFailMailer{}

	e := &store.Expense{ID: "e1", Name: "X", Amount: 100, Currency: "USD"}
	// A failing Send is logged, not returned; the call must not panic.
	svc.emailParticipants(ctx, []int64{u.ID}, map[int64]int64{u.ID: 0}, e, "added")
}

// TestCovAuthErrorBranches covers the deterministic auth error/guard paths:
// signup-disabled Register, magic-link for an unknown email under
// signup-disabled (silent success, no user created), empty magic-link email,
// admin auto-creation via magic link, and the invalid-token / missing-user
// branches of ConsumeMagicLink, ResetPassword, AdminMagicLinkForUser and
// ChangePassword.
func TestCovAuthErrorBranches(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// Register refuses when email signup is disabled.
	svc.Config.DisableEmailSignup = true
	if _, err := svc.Register(ctx, "X", "x@x.com", "password12"); !errors.Is(err, ErrSignupDisabled) {
		t.Fatalf("register disabled: got %v, want ErrSignupDisabled", err)
	}
	// Magic link for an unknown email under disabled signup is a silent success
	// that creates nothing.
	if err := svc.RequestMagicLink(ctx, "ghost@x.com"); err != nil {
		t.Fatalf("magic link disabled+unknown: %v", err)
	}
	if _, err := svc.Store.GetUserByEmail(ctx, "ghost@x.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("disabled signup should not create a user, got %v", err)
	}
	svc.Config.DisableEmailSignup = false

	// Empty magic-link email is rejected.
	if err := svc.RequestMagicLink(ctx, "   "); err == nil || !strings.Contains(err.Error(), "email is required") {
		t.Fatalf("empty magic email: got %v, want an 'email is required' error", err)
	}

	// An admin-listed unknown email is created with the ADMIN role.
	if err := svc.RequestMagicLink(ctx, "admin@example.com"); err != nil {
		t.Fatalf("admin magic link: %v", err)
	}
	au, err := svc.Store.GetUserByEmail(ctx, "admin@example.com")
	if err != nil || au.Role != "ADMIN" {
		t.Errorf("admin magic user role = %q err=%v, want ADMIN", au.Role, err)
	}

	// Invalid tokens.
	if _, err := svc.ConsumeMagicLink(ctx, "no-such-token"); err == nil ||
		!strings.Contains(err.Error(), "invalid or has expired") {
		t.Fatalf("bad magic token: got %v", err)
	}
	if err := svc.ResetPassword(ctx, "no-such-token", "newpassword1"); err == nil ||
		!strings.Contains(err.Error(), "invalid or has expired") {
		t.Fatalf("bad reset token: got %v", err)
	}

	// Missing-user lookups surface the store error.
	if _, err := svc.AdminMagicLinkForUser(ctx, 999999); err == nil {
		t.Error("AdminMagicLinkForUser should error for a missing user")
	}
	if err := svc.ChangePassword(ctx, 999999, "a", "b"); err == nil {
		t.Error("ChangePassword should error for a missing user")
	}
}

// TestCovAuthInactiveAndPromotion covers the deactivated-account guards in
// LoginPassword and ConsumeMagicLink, the no-password login branch, and the
// admin auto-promotion success path in promoteAdminIfNeeded.
func TestCovAuthInactiveAndPromotion(t *testing.T) {
	ctx := context.Background()
	svc, mailer := newTestService(t)

	// Register then deactivate: password login is blocked.
	u, err := svc.Register(ctx, "Zed", "zed@x.com", "password12")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.SetDeactivated(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LoginPassword(ctx, "zed@x.com", "password12"); !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("deactivated login: got %v, want ErrAccountInactive", err)
	}
	// Magic-link consume is blocked for a deactivated user too.
	if err := svc.RequestMagicLink(ctx, "zed@x.com"); err != nil {
		t.Fatal(err)
	}
	tok := extractToken(mailer.last(), "token=")
	if tok == "" {
		t.Fatal("no magic token emailed for zed")
	}
	if _, err := svc.ConsumeMagicLink(ctx, tok); !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("deactivated consume: got %v, want ErrAccountInactive", err)
	}

	// A user with no password hash yields ErrNoPassword on password login.
	_ = covUser(t, svc, "NoPass", "nopass@x.com")
	if _, err := svc.LoginPassword(ctx, "nopass@x.com", "whatever"); !errors.Is(err, ErrNoPassword) {
		t.Fatalf("no-password login: got %v, want ErrNoPassword", err)
	}

	// Admin auto-promotion: an existing USER whose email joins ADMIN_EMAILS is
	// promoted to ADMIN when they sign in via magic link.
	p := covUser(t, svc, "Promote", "promote@x.com")
	if p.Role == "ADMIN" {
		t.Fatal("precondition: promote user should start non-admin")
	}
	svc.Config.AdminEmails = append(svc.Config.AdminEmails, "promote@x.com")
	if err := svc.RequestMagicLink(ctx, "promote@x.com"); err != nil {
		t.Fatal(err)
	}
	ptok := extractToken(mailer.last(), "token=")
	got, err := svc.ConsumeMagicLink(ctx, ptok)
	if err != nil {
		t.Fatalf("consume promote token: %v", err)
	}
	if got.Role != "ADMIN" {
		t.Errorf("promoted role = %q, want ADMIN", got.Role)
	}
}

// TestCovImportBranches covers ImportFromSplitwise's per-entry branches: an
// empty-email friend is skipped, an admin-listed friend is created ADMIN, a
// "Non-group expenses" group is skipped, a real group is created with its
// members, and an empty-email member is skipped.
func TestCovImportBranches(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor := covUser(t, svc, "Owner", "owner@x.com")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/get_friends":
			_, _ = w.Write([]byte(`{"friends":[` +
				`{"email":"","first_name":"No","last_name":"Email"},` +
				`{"email":"admin@example.com","first_name":"Ad","last_name":"Min"},` +
				`{"email":"pal@x.com","first_name":"Pal","last_name":"X"}]}`))
		case "/get_groups":
			_, _ = w.Write([]byte(`{"groups":[` +
				`{"id":1,"name":"Non-group expenses","members":[]},` +
				`{"id":7,"name":"Trip","members":[` +
				`{"email":"","first_name":"X"},` +
				`{"email":"mate@x.com","first_name":"Mate"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	orig := splitwiseBase
	splitwiseBase = srv.URL
	defer func() { splitwiseBase = orig }()

	res, err := svc.ImportFromSplitwise(ctx, actor, "key1")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	// admin + pal = 2 friends; Trip = 1 group; empty friend + empty member = 2 skipped.
	if res.Friends != 2 || res.Groups != 1 || res.Skipped != 2 {
		t.Fatalf("import result = %+v, want Friends=2 Groups=1 Skipped=2", res)
	}
	au, err := svc.Store.GetUserByEmail(ctx, "admin@example.com")
	if err != nil || au.Role != "ADMIN" {
		t.Errorf("imported admin role = %q err=%v, want ADMIN", au.Role, err)
	}
}

// TestCovImportSwGetErrors covers the swGet failure branches and the empty-key
// guard: a blank key, an HTTP 401 (invalid key), an unexpected status, and a
// transport error against a closed local server (no external egress).
func TestCovImportSwGetErrors(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor := covUser(t, svc, "Owner", "owner@x.com")

	if _, err := svc.ImportFromSplitwise(ctx, actor, "   "); err == nil ||
		!strings.Contains(err.Error(), "API key is required") {
		t.Fatalf("empty key: got %v, want an 'API key is required' error", err)
	}

	orig := splitwiseBase
	defer func() { splitwiseBase = orig }()

	// 401 -> invalid API key.
	s401 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	splitwiseBase = s401.URL
	_, err := svc.ImportFromSplitwise(ctx, actor, "k")
	s401.Close()
	if err == nil || !strings.Contains(err.Error(), "invalid API key") {
		t.Fatalf("401: got %v, want an 'invalid API key' error", err)
	}

	// Unexpected status.
	s500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	splitwiseBase = s500.URL
	_, err = svc.ImportFromSplitwise(ctx, actor, "k")
	s500.Close()
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("500: got %v, want an 'unexpected status' error", err)
	}

	// Transport error: a server URL that is already closed refuses connections.
	sdead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := sdead.URL
	sdead.Close()
	splitwiseBase = deadURL
	if _, err := svc.ImportFromSplitwise(ctx, actor, "k"); err == nil ||
		!strings.Contains(err.Error(), "splitwise:") {
		t.Fatalf("transport error: got %v, want a wrapped 'splitwise:' error", err)
	}
}

// TestCovRecurrenceErrorBranches covers CreateRecurrence's missing-template
// error and GenerateDueRecurrences' bad-cron skip branch (a due recurrence
// whose stored cron no longer parses is warned and skipped, generating nothing).
func TestCovRecurrenceErrorBranches(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	// Valid cron but a template that does not exist -> GetExpense error.
	if _, err := svc.CreateRecurrence(ctx, a.ID, "no-such-expense", "0 0 * * *"); err == nil {
		t.Fatal("CreateRecurrence should error for a missing template")
	}

	// A real template so the recurrence row satisfies its template FK.
	tmpl, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Rent", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}
	// A due recurrence with an unparseable cron -> skipped with a warning.
	if _, err := svc.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
		CronExpression:    "not a cron",
		JobName:           "cov3-bad-cron",
		TemplateExpenseID: tmpl.ID,
		CreatedBy:         a.ID,
		NextRunAt:         sql.NullString{String: "2020-01-01T00:00:00.000Z", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	n, err := svc.GenerateDueRecurrences(ctx)
	if err != nil {
		t.Fatalf("GenerateDueRecurrences: %v", err)
	}
	if n != 0 {
		t.Fatalf("generated %d, want 0 (bad cron skipped)", n)
	}
}
