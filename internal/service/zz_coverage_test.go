package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// covUser is a small helper to create a bare user for these coverage tests. It
// is prefixed to avoid colliding with existing package helpers.
func covUser(t *testing.T, svc *Service, name, email string) *store.User {
	t.Helper()
	u, err := svc.Store.CreateUser(context.Background(), &store.User{Name: name, Email: email})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

// TestAddExpenseDirectCreatesFriendship exercises the AddExpense happy path for a
// non-group ("direct") expense: the split engine computes zero-sum participant
// rows (EQUAL of 1000 between two people = +500 / -500), the expense is persisted
// with the currency upper-cased, and direct-expense counterparts are auto-linked
// as friends (AddExpense's in.GroupID == nil branch).
func TestAddExpenseDirectCreatesFriendship(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	e, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "usd",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	})
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	if e.Amount != 1000 || e.Currency != "USD" || e.SplitType != "EQUAL" {
		t.Errorf("expense fields = %d/%s/%s, want 1000/USD/EQUAL", e.Amount, e.Currency, e.SplitType)
	}
	// Direct counterparts became friends.
	ok, err := svc.Store.AreFriends(ctx, a.ID, b.ID)
	if err != nil || !ok {
		t.Errorf("a and b should be friends after a direct expense: ok=%v err=%v", ok, err)
	}
	// Participant rows sum to zero.
	parts, err := svc.Store.GetParticipants(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, p := range parts {
		sum += p.Amount
	}
	if len(parts) != 2 || sum != 0 {
		t.Errorf("participants = %d rows summing %d, want 2 rows summing 0", len(parts), sum)
	}
}

// TestAddExpenseNonMemberRejected pins AddExpense's group-membership guard: every
// participant (and the payer) must belong to the target group, otherwise
// ErrNotMember is returned and nothing is persisted.
func TestAddExpenseNonMemberRejected(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com") // NOT a group member
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})

	_, err := svc.AddExpense(ctx, ExpenseInput{
		Name: "Dinner", Total: 1000, Method: split.EQUAL, Currency: "USD",
		ExpenseDate: "2026-01-10", PaidBy: a.ID, ActorID: a.ID, GroupID: &g.ID,
		Lines: []split.Line{{UserID: a.ID}, {UserID: b.ID}},
	})
	if err != ErrNotMember {
		t.Fatalf("non-member participant: got %v, want ErrNotMember", err)
	}
	var n int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses`).Scan(&n)
	if n != 0 {
		t.Errorf("expense persisted despite ErrNotMember: %d rows", n)
	}
}

// TestSettleRecordsDirectionAndRejectsNonPositive covers Settle: a non-positive
// amount is refused, and a positive settlement is stored as a SETTLEMENT expense
// paid by the sender with sender +amount / receiver -amount participant rows
// (sender pays receiver).
func TestSettleRecordsDirectionAndRejectsNonPositive(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	sender := covUser(t, svc, "S", "s@x.com")
	receiver := covUser(t, svc, "R", "r@x.com")

	if _, err := svc.Settle(ctx, sender.ID, receiver.ID, 0, "USD", nil, "2026-01-10", sender.ID); err == nil {
		t.Error("zero settlement amount should error")
	}
	if _, err := svc.Settle(ctx, sender.ID, receiver.ID, -5, "USD", nil, "2026-01-10", sender.ID); err == nil {
		t.Error("negative settlement amount should error")
	}

	e, err := svc.Settle(ctx, sender.ID, receiver.ID, 2500, "usd", nil, "2026-01-10", sender.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if e.SplitType != "SETTLEMENT" || e.PaidBy != sender.ID || e.Amount != 2500 || e.Currency != "USD" {
		t.Errorf("settlement = %s paidBy=%d amt=%d cur=%s, want SETTLEMENT/%d/2500/USD",
			e.SplitType, e.PaidBy, e.Amount, e.Currency, sender.ID)
	}
	parts, err := svc.Store.GetParticipants(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]int64{}
	for _, p := range parts {
		got[p.UserID] = p.Amount
	}
	if got[sender.ID] != 2500 || got[receiver.ID] != -2500 {
		t.Errorf("settlement direction = sender %d / receiver %d, want +2500 / -2500", got[sender.ID], got[receiver.ID])
	}
}

// TestGetRateSameCurrencyCacheAndProvider covers GetRate: identical currencies
// short-circuit to "1", a cache miss falls back to the provider and then caches
// the result, and GetRates batches several targets from one base.
func TestGetRateSameCurrencyCacheAndProvider(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	svc.Rates = fakeRates{rate: "0.9"}

	if r, err := svc.GetRate(ctx, "usd", "USD", ""); err != nil || r != "1" {
		t.Fatalf("same-currency rate = %q %v, want 1", r, err)
	}
	// Cache miss -> provider -> cached.
	r, err := svc.GetRate(ctx, "usd", "eur", "2026-02-02")
	if err != nil || r != "0.9" {
		t.Fatalf("provider rate = %q %v, want 0.9", r, err)
	}
	if cached, err := svc.Store.GetCachedRate(ctx, "USD", "EUR", "2026-02-02"); err != nil || cached != "0.9" {
		t.Errorf("rate not cached: %q %v", cached, err)
	}
	// Batch lookup.
	m, err := svc.GetRates(ctx, "USD", []string{"EUR", "USD"}, "2026-02-02")
	if err != nil {
		t.Fatal(err)
	}
	if m["EUR"] != "0.9" || m["USD"] != "1" {
		t.Errorf("GetRates = %v, want EUR=0.9 USD=1", m)
	}
}

// TestAddFriendByEmail covers the three AddFriendByEmail branches: linking an
// existing user, refusing self-add, and the invites-disabled failure from
// findOrCreateUser when the email is unknown.
func TestAddFriendByEmail(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	if err := svc.AddFriendByEmail(ctx, a.ID, "b@x.com"); err != nil {
		t.Fatalf("add existing friend: %v", err)
	}
	if ok, _ := svc.Store.AreFriends(ctx, a.ID, b.ID); !ok {
		t.Error("a and b should be friends")
	}
	// Can't add yourself.
	if err := svc.AddFriendByEmail(ctx, a.ID, "a@x.com"); err == nil || !strings.Contains(err.Error(), "yourself") {
		t.Errorf("self-add: got %v, want a 'yourself' error", err)
	}
	// Unknown email with invites disabled is refused.
	svc.Config.EnableSendingInvites = false
	if err := svc.AddFriendByEmail(ctx, a.ID, "ghost@x.com"); err == nil || !strings.Contains(err.Error(), "invites are disabled") {
		t.Errorf("invites-disabled: got %v, want an 'invites are disabled' error", err)
	}
}

// TestToggleHiddenFriend covers ToggleHiddenFriend flipping a friend id onto and
// back off the hidden list, and persisting each change to the store.
func TestToggleHiddenFriend(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")

	if err := svc.ToggleHiddenFriend(ctx, u, 99); err != nil {
		t.Fatal(err)
	}
	if len(u.HiddenFriendIDs) != 1 || u.HiddenFriendIDs[0] != 99 {
		t.Errorf("after hide: %v, want [99]", u.HiddenFriendIDs)
	}
	reloaded, err := svc.Store.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.HiddenFriendIDs) != 1 || reloaded.HiddenFriendIDs[0] != 99 {
		t.Errorf("hidden list not persisted: %v", reloaded.HiddenFriendIDs)
	}
	// Toggling again removes it.
	if err := svc.ToggleHiddenFriend(ctx, u, 99); err != nil {
		t.Fatal(err)
	}
	if len(u.HiddenFriendIDs) != 0 {
		t.Errorf("after unhide: %v, want empty", u.HiddenFriendIDs)
	}
}

// TestInviteToGroup covers InviteToGroup: a non-member cannot invite, while a
// member can add an existing user to the group.
func TestInviteToGroup(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com") // group creator -> auto member
	b := covUser(t, svc, "B", "b@x.com") // invitee (existing user)
	c := covUser(t, svc, "C", "c@x.com") // outsider
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})

	// Outsider cannot invite.
	if err := svc.InviteToGroup(ctx, c, g.ID, "b@x.com"); err == nil || !strings.Contains(err.Error(), "only members") {
		t.Errorf("outsider invite: got %v, want an 'only members' error", err)
	}
	// Member adds an existing user.
	if err := svc.InviteToGroup(ctx, a, g.ID, "b@x.com"); err != nil {
		t.Fatalf("member invite: %v", err)
	}
	if ok, _ := svc.Store.IsGroupMember(ctx, g.ID, b.ID); !ok {
		t.Error("b should be a group member after invite")
	}
}

// TestCreateRecurrenceValidation covers CreateRecurrence: a malformed cron
// expression is rejected, a currency-conversion template cannot recur, and a
// valid template + cron produces a recurrence that ListRecurrences returns and
// DeleteRecurrence removes.
func TestCreateRecurrenceValidation(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	// Bad cron.
	if _, err := svc.CreateRecurrence(ctx, a.ID, "no-such-expense", "not a cron"); err == nil ||
		!strings.Contains(err.Error(), "invalid cron expression") {
		t.Errorf("bad cron: got %v, want an 'invalid cron expression' error", err)
	}

	// Currency-conversion template cannot recur.
	conv, err := svc.CreateConversionExact(ctx, a.ID, a.ID, b.ID, 10000, 7830, "SGD", "USD", "2026-01-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRecurrence(ctx, a.ID, conv.ID, "0 0 * * *"); err == nil ||
		!strings.Contains(err.Error(), "cannot recur") {
		t.Errorf("conversion recur: got %v, want a 'cannot recur' error", err)
	}

	// Valid template + cron.
	tmpl, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Rent", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := svc.CreateRecurrence(ctx, a.ID, tmpl.ID, "0 0 * * *")
	if err != nil {
		t.Fatalf("CreateRecurrence: %v", err)
	}
	if rec.CronExpression != "0 0 * * *" || !rec.NextRunAt.Valid {
		t.Errorf("recurrence = %+v, want cron set and NextRunAt valid", rec)
	}
	list, err := svc.ListRecurrences(ctx, a.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRecurrences = %d %v, want 1", len(list), err)
	}
	if err := svc.DeleteRecurrence(ctx, rec.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.ListRecurrences(ctx, a.ID); len(list) != 0 {
		t.Errorf("after delete: %d recurrences, want 0", len(list))
	}
}

// TestGenerateDueRecurrences covers GenerateDueRecurrences: a due recurrence
// generates one fresh expense from its template (carrying the recurrence id) and
// is advanced to a future next-run time.
func TestGenerateDueRecurrences(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	tmpl, err := svc.Store.CreateExpense(ctx, &store.Expense{
		Name: "Rent", Category: "general", Amount: 1000, SplitType: "EQUAL",
		ExpenseDate: "2026-01-10", Currency: "USD", PaidBy: a.ID, AddedBy: a.ID,
	}, []store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	if err != nil {
		t.Fatal(err)
	}
	// A recurrence already past due (next_run_at well in the past).
	if _, err := svc.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
		CronExpression:    "0 0 * * *",
		JobName:           "cov-job-1",
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
	if n != 1 {
		t.Fatalf("generated %d, want 1", n)
	}
	var gen int
	_ = svc.Store.DB.QueryRow(`SELECT count(*) FROM expenses WHERE recurrence_id IS NOT NULL`).Scan(&gen)
	if gen != 1 {
		t.Errorf("generated expenses with recurrence_id = %d, want 1", gen)
	}
}

// TestPreviewArchive covers PreviewArchive counting collapsible expenses per
// currency for both a direct pair and a group target.
func TestPreviewArchive(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")

	// Two direct SGD expenses + one USD, all before the cutoff.
	mkExp(t, svc, "D1", 1000, "SGD", "2026-01-10T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mkExp(t, svc, "D2", 600, "SGD", "2026-02-10T00:00:00Z", b.ID, nil,
		[]store.ExpenseParticipant{{UserID: b.ID, Amount: 300}, {UserID: a.ID, Amount: -300}})
	mkExp(t, svc, "D3", 400, "USD", "2026-02-11T00:00:00Z", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 200}, {UserID: b.ID, Amount: -200}})

	counts, err := svc.PreviewArchive(ctx, a.ID, &b.ID, nil, "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if counts["SGD"] != 2 || counts["USD"] != 1 {
		t.Errorf("direct preview = %v, want SGD=2 USD=1", counts)
	}

	// Group target.
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	_ = svc.Store.AddGroupMember(ctx, g.ID, b.ID)
	mkExp(t, svc, "G1", 900, "EUR", "2026-01-10T00:00:00Z", a.ID, &g.ID,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 450}, {UserID: b.ID, Amount: -450}})
	gCounts, err := svc.PreviewArchive(ctx, a.ID, nil, &g.ID, "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if gCounts["EUR"] != 1 {
		t.Errorf("group preview = %v, want EUR=1", gCounts)
	}
}

// TestArchiveDirectRejectsSelf pins ArchiveDirect's self-guard.
func TestArchiveDirectRejectsSelf(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	if _, err := svc.ArchiveDirect(ctx, a.ID, a.ID, "2026-03-01"); err == nil ||
		!strings.Contains(err.Error(), "yourself") {
		t.Errorf("self archive: got %v, want a 'yourself' error", err)
	}
}

// TestArchiveGroupNonMember pins ArchiveGroup's membership guard.
func TestArchiveGroupNonMember(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	outsider := covUser(t, svc, "O", "o@x.com")
	g, _ := svc.Store.CreateGroup(ctx, &store.Group{Name: "G", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if _, err := svc.ArchiveGroup(ctx, outsider.ID, g.ID, "2026-03-01"); err != ErrNotGroupMember {
		t.Fatalf("non-member archive: got %v, want ErrNotGroupMember", err)
	}
}

// TestAdminMagicLinkForUser covers AdminMagicLinkForUser returning a usable
// magic-link URL for an existing user.
func TestAdminMagicLinkForUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")
	link, err := svc.AdminMagicLinkForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("AdminMagicLinkForUser: %v", err)
	}
	if !strings.Contains(link, "/auth/magic?token=") || !strings.HasPrefix(link, svc.Config.BaseURL) {
		t.Errorf("link = %q, want it under BaseURL with a magic token", link)
	}
	tok := extractToken(link, "token=")
	if tok == "" {
		t.Fatal("no token in magic link")
	}
	// The minted token is a real, consumable magic-link token.
	got, err := svc.ConsumeMagicLink(ctx, tok)
	if err != nil || got.ID != u.ID {
		t.Errorf("consume minted token: got %+v err=%v", got, err)
	}
}

// TestExportUserData covers ExportUserData producing valid JSON that includes the
// account's email.
func TestExportUserData(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	_ = svc.Store.AddFriend(ctx, a.ID, b.ID)

	blob, err := svc.ExportUserData(ctx, a.ID)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if !json.Valid(blob) {
		t.Fatal("export is not valid JSON")
	}
	var payload map[string]any
	if err := json.Unmarshal(blob, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["user"]; !ok {
		t.Errorf("export missing 'user' key: %v", payload)
	}
	if !strings.Contains(string(blob), "a@x.com") {
		t.Error("export should contain the account email")
	}
}

// TestBankDisabled covers the bank service under the default (unconfigured)
// provider: BankEnabled is false, link/connect fail loudly, an unconnected sync
// is refused, and the cache lookups return empty.
func TestBankDisabled(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	u := covUser(t, svc, "U", "u@x.com")

	if svc.BankEnabled() {
		t.Error("bank should be disabled with no Plaid config")
	}
	if _, err := svc.CreateBankLinkToken(ctx, u); err == nil {
		t.Error("CreateBankLinkToken should fail when bank is disabled")
	}
	if err := svc.ConnectBank(ctx, u, "public-token"); err == nil {
		t.Error("ConnectBank should fail when bank is disabled")
	}
	if _, err := svc.SyncBankTransactions(ctx, u); err == nil ||
		!strings.Contains(err.Error(), "no bank account connected") {
		t.Errorf("sync without connection: got %v, want a 'no bank account connected' error", err)
	}
	if txns := svc.CachedTransactions(ctx, u.ID); len(txns) != 0 {
		t.Errorf("cached transactions = %d, want 0", len(txns))
	}
	if _, ok := svc.FindTransaction(ctx, u.ID, "nope"); ok {
		t.Error("FindTransaction should not find a nonexistent id")
	}
}
