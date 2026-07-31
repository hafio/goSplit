package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/bank"
	"github.com/hafio/gosplit/internal/store"
)

// covMultipartReq builds a POST request whose body is a single multipart form
// field. It is used to drive UpdateAvatar without touching the network.
func covMultipartReq(t *testing.T, field, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/avatar", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// TestCovUpdateAvatar exercises every branch of UpdateAvatar: a non-multipart
// body and a missing "avatar" field are both no-ops (return nil, no image set);
// an oversized file and an unsupported extension fail loudly; and a valid image
// is written under UploadDir with the user's Image path pointed at it.
func TestCovUpdateAvatar(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// Non-multipart body: ParseMultipartForm errors -> nil, nothing set.
	u := &store.User{}
	nonMulti := httptest.NewRequest(http.MethodPost, "/settings/avatar", strings.NewReader("x"))
	if err := svc.UpdateAvatar(ctx, u, nonMulti); err != nil {
		t.Fatalf("non-multipart should be a no-op, got %v", err)
	}
	if u.Image.Valid {
		t.Error("non-multipart body should not set an image")
	}

	// Multipart body without an "avatar" field: FormFile errors -> nil.
	u = &store.User{}
	svc.Config.UploadMaxFileSizeMB = 10
	if err := svc.UpdateAvatar(ctx, u, covMultipartReq(t, "other", "x.png", "data")); err != nil {
		t.Fatalf("missing avatar field should be a no-op, got %v", err)
	}
	if u.Image.Valid {
		t.Error("missing avatar field should not set an image")
	}

	// Oversized file: with a 0 MB limit any non-empty file exceeds it.
	u = &store.User{}
	svc.Config.UploadMaxFileSizeMB = 0
	if err := svc.UpdateAvatar(ctx, u, covMultipartReq(t, "avatar", "big.png", "data")); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized upload: got %v, want an 'exceeds' error", err)
	}

	// Unsupported extension (size check passes with a real limit).
	u = &store.User{}
	svc.Config.UploadMaxFileSizeMB = 10
	if err := svc.UpdateAvatar(ctx, u, covMultipartReq(t, "avatar", "note.txt", "data")); err == nil ||
		!strings.Contains(err.Error(), "unsupported image type") {
		t.Fatalf("bad extension: got %v, want an 'unsupported image type' error", err)
	}

	// Happy path: valid PNG lands under UploadDir and the user's Image is set.
	u = &store.User{}
	svc.Config.UploadDir = filepath.Join(t.TempDir(), "uploads")
	if err := svc.UpdateAvatar(ctx, u, covMultipartReq(t, "avatar", "pic.PNG", "imagebytes")); err != nil {
		t.Fatalf("valid upload: %v", err)
	}
	if !u.Image.Valid || !strings.HasPrefix(u.Image.String, "/uploads/") {
		t.Fatalf("image path = %q, want a /uploads/ path", u.Image.String)
	}
	name := strings.TrimPrefix(u.Image.String, "/uploads/")
	if !strings.HasSuffix(strings.ToLower(name), ".png") {
		t.Errorf("stored file %q should keep the .png extension", name)
	}
	if _, err := os.Stat(filepath.Join(svc.Config.UploadDir, name)); err != nil {
		t.Errorf("stored avatar not found on disk: %v", err)
	}
}

// covBank is an in-memory bank.Provider used to drive the bank service without a
// live Plaid backend. Flags let a test steer the exchange/fetch error branches.
type covBank struct {
	token    string
	txns     []bank.Transaction
	failLink bool
	failExch bool
	failTxns bool
}

func (covBank) Name() string { return "cov" }
func (covBank) Enabled() bool { return true }
func (b covBank) CreateLinkToken(context.Context, string) (string, error) {
	if b.failLink {
		return "", errors.New("link failed")
	}
	return "link-token", nil
}
func (b covBank) ExchangePublicToken(context.Context, string) (string, error) {
	if b.failExch {
		return "", errors.New("exchange failed")
	}
	return b.token, nil
}
func (b covBank) FetchTransactions(context.Context, string, string, string) ([]bank.Transaction, error) {
	if b.failTxns {
		return nil, errors.New("fetch failed")
	}
	return b.txns, nil
}

// TestCovBankConnectedFlow covers the enabled-provider path across the bank
// service: link-token creation, exchanging a public token then persisting it,
// syncing + caching transactions, and reading them back via CachedTransactions
// and FindTransaction (hit and miss).
func TestCovBankConnectedFlow(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")
	txns := []bank.Transaction{
		{ID: "t1", Name: "Coffee", AmountMinor: 500, Currency: "USD", Date: "2026-01-10"},
		{ID: "t2", Name: "Books", AmountMinor: 2000, Currency: "USD", Date: "2026-01-11"},
	}
	svc.Bank = covBank{token: "access-1", txns: txns}

	if !svc.BankEnabled() {
		t.Fatal("bank should be enabled with the fake provider")
	}
	if tok, err := svc.CreateBankLinkToken(ctx, u); err != nil || tok != "link-token" {
		t.Fatalf("CreateBankLinkToken = %q %v, want link-token", tok, err)
	}
	if err := svc.ConnectBank(ctx, u, "public-1"); err != nil {
		t.Fatalf("ConnectBank: %v", err)
	}
	got, err := svc.SyncBankTransactions(ctx, u)
	if err != nil {
		t.Fatalf("SyncBankTransactions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("synced %d transactions, want 2", len(got))
	}
	if cached := svc.CachedTransactions(ctx, u.ID); len(cached) != 2 {
		t.Errorf("cached %d transactions, want 2", len(cached))
	}
	if tx, ok := svc.FindTransaction(ctx, u.ID, "t2"); !ok || tx.Name != "Books" {
		t.Errorf("FindTransaction(t2) = %+v ok=%v, want Books", tx, ok)
	}
	if _, ok := svc.FindTransaction(ctx, u.ID, "missing"); ok {
		t.Error("FindTransaction should miss an unknown id")
	}
}

// TestCovBankProviderErrors pins the two provider-error branches reachable once a
// provider is present: ExchangePublicToken failing aborts ConnectBank, and
// FetchTransactions failing aborts SyncBankTransactions after a connection.
func TestCovBankProviderErrors(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")

	svc.Bank = covBank{token: "access-1", failExch: true}
	if err := svc.ConnectBank(ctx, u, "public-1"); err == nil ||
		!strings.Contains(err.Error(), "exchange failed") {
		t.Fatalf("ConnectBank exchange error: got %v", err)
	}

	// Connect successfully, then make the fetch fail on sync.
	svc.Bank = covBank{token: "access-1", failTxns: true}
	if err := svc.ConnectBank(ctx, u, "public-1"); err != nil {
		t.Fatalf("ConnectBank: %v", err)
	}
	if _, err := svc.SyncBankTransactions(ctx, u); err == nil ||
		!strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("SyncBankTransactions fetch error: got %v", err)
	}
}

// TestCovInviteSendsPendingInvites covers the invite branches of the friends
// service: a brand-new email creates a pending user and emails an invite (for
// both AddFriendByEmail and InviteToGroup), an admin-listed email is created
// with the ADMIN role, and an empty email is rejected.
func TestCovInviteSendsPendingInvites(t *testing.T) {
	ctx := context.Background()
	svc, mailer := newTestService(t) // EnableSendingInvites is true in newTestService
	owner := covUser(t, svc, "Owner", "owner@x.com")

	// Empty email is rejected by findOrCreateUser.
	if err := svc.AddFriendByEmail(ctx, owner.ID, "   "); err == nil ||
		!strings.Contains(err.Error(), "email is required") {
		t.Fatalf("empty email: got %v, want an 'email is required' error", err)
	}

	// New email -> pending user created + invite emailed.
	before := len(mailer.msgs)
	if err := svc.AddFriendByEmail(ctx, owner.ID, "newfriend@x.com"); err != nil {
		t.Fatalf("AddFriendByEmail (invite): %v", err)
	}
	if len(mailer.msgs) != before+1 || !strings.Contains(mailer.last(), "added you on GoSplit") {
		t.Errorf("expected one friend-invite email, got %d msgs last=%q", len(mailer.msgs)-before, mailer.last())
	}
	invited, err := svc.Store.GetUserByEmail(ctx, "newfriend@x.com")
	if err != nil {
		t.Fatalf("invited user not created: %v", err)
	}
	if invited.Role != "USER" {
		t.Errorf("invited role = %q, want USER", invited.Role)
	}

	// Admin-listed email is created with the ADMIN role.
	if err := svc.AddFriendByEmail(ctx, owner.ID, "admin@example.com"); err != nil {
		t.Fatalf("AddFriendByEmail (admin): %v", err)
	}
	adminU, err := svc.Store.GetUserByEmail(ctx, "admin@example.com")
	if err != nil || adminU.Role != "ADMIN" {
		t.Errorf("admin invite role = %q err=%v, want ADMIN", adminU.Role, err)
	}

	// InviteToGroup with a new email -> pending member + invite naming the group.
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: owner.ID, DefaultCurrency: "USD"})
	before = len(mailer.msgs)
	if err := svc.InviteToGroup(ctx, owner, g.ID, "groupie@x.com"); err != nil {
		t.Fatalf("InviteToGroup (invite): %v", err)
	}
	if len(mailer.msgs) != before+1 || !strings.Contains(mailer.last(), "Trip") {
		t.Errorf("expected one group-invite email naming the group, got last=%q", mailer.last())
	}
	member, err := svc.Store.GetUserByEmail(ctx, "groupie@x.com")
	if err != nil {
		t.Fatalf("group invitee not created: %v", err)
	}
	if ok, _ := svc.Store.IsGroupMember(ctx, g.ID, member.ID); !ok {
		t.Error("group invitee should be a member")
	}
}

// TestCovEmailParticipants drives emailParticipants directly (bypassing the
// fire-and-forget notifyExpense goroutine) to cover the owe / owed / unchanged
// message branches and the skip when a recipient can't be loaded.
func TestCovEmailParticipants(t *testing.T) {
	ctx := context.Background()
	svc, mailer := newTestService(t) // synchronous mail
	ower := covUser(t, svc, "Ower", "ower@x.com")
	owed := covUser(t, svc, "Owed", "owed@x.com")
	flat := covUser(t, svc, "Flat", "flat@x.com")

	e := &store.Expense{ID: "exp-1", Name: "Dinner", Amount: 1000, Currency: "USD"}
	amounts := map[int64]int64{ower.ID: -500, owed.ID: 500, flat.ID: 0}
	// A non-existent user id (999999) exercises the GetUser-error skip.
	svc.emailParticipants(ctx, []int64{ower.ID, owed.ID, flat.ID, 999999}, amounts, e, "added")

	if len(mailer.msgs) != 3 {
		t.Fatalf("sent %d emails, want 3 (unknown recipient skipped)", len(mailer.msgs))
	}
	var sawOwe, sawOwed, sawFlat bool
	for _, m := range mailer.msgs {
		switch {
		case strings.Contains(m.Body, "You owe"):
			sawOwe = true
		case strings.Contains(m.Body, "You are owed"):
			sawOwed = true
		case strings.Contains(m.Body, "balance is unchanged"):
			sawFlat = true
		}
	}
	if !sawOwe || !sawOwed || !sawFlat {
		t.Errorf("expected owe/owed/unchanged bodies, got owe=%v owed=%v flat=%v", sawOwe, sawOwed, sawFlat)
	}
}

// TestCovPushGating covers pushToUsers early-return gates that are reachable
// without a live push backend: an empty recipient list, the default disabled
// Sender, and a nil Sender all return without attempting delivery.
func TestCovPushGating(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")
	e := &store.Expense{ID: "exp-1", Name: "Dinner", Amount: 1000, Currency: "USD"}

	// Empty recipients -> early return.
	svc.pushToUsers(ctx, nil, "title", e)
	// Default Sender is disabled (no VAPID keys) -> early return.
	svc.pushToUsers(ctx, []int64{u.ID}, "title", e)
	// Nil Sender -> early return.
	svc.Push = nil
	svc.pushToUsers(ctx, []int64{u.ID}, "title", e)
}

// covErrRates is a currency.Provider whose Rate lookup always fails, used to
// cover the provider-error branches of GetRate and CreateConversion.
type covErrRates struct{}

func (covErrRates) Name() string { return "err" }
func (covErrRates) Rate(context.Context, string, string, string) (string, error) {
	return "", errors.New("rate unavailable")
}

// TestCovConversionErrors covers CreateConversion's guard and provider-failure
// branches (same currency, non-positive amount, provider error, unparseable
// rate) and GetRate's provider-error path.
func TestCovConversionErrors(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	// Same currency and non-positive amount are rejected before any lookup.
	if _, err := svc.CreateConversion(ctx, a.ID, a.ID, b.ID, 1000, "USD", "USD", "2026-01-01", nil); err == nil ||
		!strings.Contains(err.Error(), "must differ") {
		t.Errorf("same-currency: got %v, want a 'must differ' error", err)
	}
	if _, err := svc.CreateConversion(ctx, a.ID, a.ID, b.ID, 0, "USD", "EUR", "2026-01-01", nil); err == nil ||
		!strings.Contains(err.Error(), "must be positive") {
		t.Errorf("zero amount: got %v, want a 'must be positive' error", err)
	}

	// Provider failure bubbles up through GetRate.
	svc.Rates = covErrRates{}
	if _, err := svc.CreateConversion(ctx, a.ID, a.ID, b.ID, 1000, "USD", "EUR", "2026-03-03", nil); err == nil {
		t.Error("provider error should abort CreateConversion")
	}
	if _, err := svc.GetRate(ctx, "USD", "GBP", "2026-03-03"); err == nil {
		t.Error("GetRate should surface a provider error")
	}

	// A provider that returns an unparseable rate fails in currency.Convert.
	svc.Rates = fakeRates{rate: "not-a-number"}
	if _, err := svc.CreateConversion(ctx, a.ID, a.ID, b.ID, 1000, "USD", "JPY", "2026-04-04", nil); err == nil {
		t.Error("unparseable rate should abort CreateConversion via Convert")
	}
}
