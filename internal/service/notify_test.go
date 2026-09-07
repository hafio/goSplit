package service

import (
	"context"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// --- fixtures -------------------------------------------------------------

// covNotif builds a delivery-shaped notification for one recipient. amount is
// that recipient's signed share, which is what selects the share-line branch.
func covNotif(userID, amount int64) *store.Notification {
	return &store.Notification{
		UserID: userID, ActorID: 1, Kind: KindExpenseAdded,
		EntityType: entityExpense, EntityID: "exp-1",
		Title: "Dinner", Amount: amount, Currency: "USD",
	}
}

// covUserMap resolves the recipients of rows the way deliver does, skipping ids
// with no account so the caller can exercise that branch.
func covUserMap(t *testing.T, svc *Service, rows []*store.Notification) map[int64]*store.User {
	t.Helper()
	users := map[int64]*store.User{}
	for _, n := range rows {
		if u, err := svc.Store.GetUser(context.Background(), n.UserID); err == nil {
			users[n.UserID] = u
		}
	}
	return users
}

// covOptInEmail turns on the expense-email opt-in, which is off by default.
func covOptInEmail(t *testing.T, svc *Service, u *store.User) *store.User {
	t.Helper()
	u.EmailExpenseNotify = true
	if err := svc.Store.UpdateProfile(context.Background(), u); err != nil {
		t.Fatalf("opt %s into email: %v", u.Email, err)
	}
	return u
}

// covNotifications reads a user's notifications, failing the test on error.
func covNotifications(t *testing.T, svc *Service, userID int64) []*store.Notification {
	t.Helper()
	ns, err := svc.Store.ListNotifications(context.Background(), userID, 0)
	if err != nil {
		t.Fatalf("list notifications for %d: %v", userID, err)
	}
	return ns
}

// covPair builds two users and a direct expense input between them.
func covPair(t *testing.T, svc *Service) (*store.User, *store.User) {
	t.Helper()
	return covUser(t, svc, "Actor", "actor@x.com"), covUser(t, svc, "Other", "other@x.com")
}

func covInput(actor, payer, other *store.User, total int64) ExpenseInput {
	return ExpenseInput{
		Name: "Dinner", Category: "general", Total: total, Method: split.EQUAL,
		Currency: "USD", ExpenseDate: "2026-03-01", PaidBy: payer.ID, ActorID: actor.ID,
		Lines: []split.Line{{UserID: payer.ID}, {UserID: other.ID}},
	}
}

// --- events ---------------------------------------------------------------

// TestNotifyAddExpenseRecordsForOthersOnly asserts the shape every event
// shares: one row per participant except the actor, linked to the expense with
// the recipient's own signed share.
func TestNotifyAddExpenseRecordsForOthersOnly(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor, other := covPair(t, svc)

	e, err := svc.AddExpense(ctx, covInput(actor, actor, other, 1000))
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}

	if got := covNotifications(t, svc, actor.ID); len(got) != 0 {
		t.Errorf("actor got %d notifications, want 0", len(got))
	}
	ns := covNotifications(t, svc, other.ID)
	if len(ns) != 1 {
		t.Fatalf("recipient got %d notifications, want 1", len(ns))
	}
	n := ns[0]
	switch {
	case n.Kind != KindExpenseAdded:
		t.Errorf("kind = %q, want %q", n.Kind, KindExpenseAdded)
	case n.EntityType != entityExpense || n.EntityID != e.ID:
		t.Errorf("link = %s/%s, want expense/%s", n.EntityType, n.EntityID, e.ID)
	case n.ActorID != actor.ID:
		t.Errorf("actor = %d, want %d", n.ActorID, actor.ID)
	case n.Title != "Dinner":
		t.Errorf("title = %q, want %q", n.Title, "Dinner")
	case n.Amount != -500:
		t.Errorf("amount = %d, want -500 (the recipient's own share)", n.Amount)
	case n.Currency != "USD":
		t.Errorf("currency = %q, want USD", n.Currency)
	case n.IsRead():
		t.Error("a new notification must be unread")
	}
}

// TestNotifyEventKinds covers the four events beyond AddExpense, each of which
// notified nobody at all before this feature.
func TestNotifyEventKinds(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor, other := covPair(t, svc)

	// Edit: MoveExpense is the in-place edit path.
	e, err := svc.AddExpense(ctx, covInput(actor, actor, other, 1000))
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	in := covInput(actor, actor, other, 2000)
	in.Version = e.Version
	if _, err := svc.MoveExpense(ctx, e.ID, in, true); err != nil {
		t.Fatalf("MoveExpense: %v", err)
	}

	// Delete: participants have to be fetched, they are not in scope there.
	e2, err := svc.AddExpense(ctx, covInput(actor, actor, other, 500))
	if err != nil {
		t.Fatalf("AddExpense 2: %v", err)
	}
	if err := svc.DeleteExpense(ctx, e2.ID, actor.ID, e2.Version); err != nil {
		t.Fatalf("DeleteExpense: %v", err)
	}

	// Settle and settlement edit.
	s0, err := svc.Settle(ctx, actor.ID, other.ID, 250, "USD", nil, "2026-03-02", actor.ID)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if _, err := svc.UpdateSettlement(ctx, s0.ID, SettlementInput{
		Amount: 300, ActorID: actor.ID, Version: s0.Version,
	}, false); err != nil {
		t.Fatalf("UpdateSettlement: %v", err)
	}

	want := map[string]int{
		KindExpenseAdded:      2,
		KindExpenseUpdated:    1,
		KindExpenseDeleted:    1,
		KindSettlementAdded:   1,
		KindSettlementUpdated: 1,
	}
	got := map[string]int{}
	for _, n := range covNotifications(t, svc, other.ID) {
		got[n.Kind]++
	}
	for kind, n := range want {
		if got[kind] != n {
			t.Errorf("kind %q: %d notifications, want %d", kind, got[kind], n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("recorded kinds = %v, want exactly %v", got, want)
	}
}

// TestNotifyDeletedExpenseKeepsItsNotification proves the no-foreign-key link:
// a soft delete neither removes the notification nor breaks its reference.
func TestNotifyDeletedExpenseKeepsItsNotification(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor, other := covPair(t, svc)

	e, err := svc.AddExpense(ctx, covInput(actor, actor, other, 1000))
	if err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	if err := svc.DeleteExpense(ctx, e.ID, actor.ID, e.Version); err != nil {
		t.Fatalf("DeleteExpense: %v", err)
	}
	for _, n := range covNotifications(t, svc, other.ID) {
		if n.EntityID != e.ID {
			t.Errorf("notification lost its link: entity_id = %q, want %q", n.EntityID, e.ID)
		}
	}
}

// TestSettleAllRecordsPerTransfer asserts the settle-up batch: a row per
// settlement, so the bell stays per-transfer and each entry deep-links to its
// own settlement even though the batch pushes only once.
func TestSettleAllRecordsPerTransfer(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	c := covUser(t, svc, "C", "c@x.com")
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, u := range []*store.User{a, b, c} {
		if err := svc.Store.AddGroupMember(ctx, g.ID, u.ID); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	settled, err := svc.SettleAll(ctx, []Transfer{
		{FromID: b.ID, ToID: a.ID, Amount: 500, Currency: "USD"},
		{FromID: c.ID, ToID: a.ID, Amount: 300, Currency: "USD"},
	}, g.ID, "2026-03-03", a.ID)
	if err != nil {
		t.Fatalf("SettleAll: %v", err)
	}
	if len(settled) != 2 {
		t.Fatalf("recorded %d settlements, want 2", len(settled))
	}
	// a is the actor on both, so a is told nothing; b and c are each on exactly
	// one transfer.
	if got := covNotifications(t, svc, a.ID); len(got) != 0 {
		t.Errorf("actor got %d notifications, want 0", len(got))
	}
	for _, u := range []*store.User{b, c} {
		ns := covNotifications(t, svc, u.ID)
		if len(ns) != 1 {
			t.Fatalf("%s got %d notifications, want 1", u.Name, len(ns))
		}
		if ns[0].Kind != KindSettlementAdded {
			t.Errorf("%s: kind = %q, want %q", u.Name, ns[0].Kind, KindSettlementAdded)
		}
	}
}

// TestSettleAllEmptyRecordsNothing covers the no-transfers short circuit: a
// second settle-up of an already-settled group must record nothing.
func TestSettleAllEmptyRecordsNothing(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	settled, err := svc.SettleAll(ctx, nil, 1, "2026-03-03", a.ID)
	if err != nil {
		t.Fatalf("SettleAll: %v", err)
	}
	if len(settled) != 0 {
		t.Errorf("recorded %d settlements, want 0", len(settled))
	}
}

// TestSettleAllStopsOnError asserts a bad leg is reported rather than skipped,
// and that the legs already written are handed back so the caller can say what
// happened.
func TestSettleAllStopsOnError(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	// A real group: expenses.group_id is a foreign key, so a made-up id would
	// fail on the constraint rather than on the amount under test.
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	settled, err := svc.SettleAll(ctx, []Transfer{
		{FromID: b.ID, ToID: a.ID, Amount: 500, Currency: "USD"},
		{FromID: b.ID, ToID: a.ID, Amount: 0, Currency: "USD"}, // rejected
	}, g.ID, "2026-03-03", a.ID)
	if err == nil {
		t.Fatal("a non-positive amount must fail the batch")
	}
	if len(settled) != 1 {
		t.Errorf("returned %d completed settlements, want 1", len(settled))
	}
}

// --- channels -------------------------------------------------------------

// TestNotifyEmailIsOptIn is the regression guard for the decision that in-app is
// the default channel: mail goes out only to recipients who asked for it.
func TestNotifyEmailIsOptIn(t *testing.T) {
	ctx := context.Background()
	svc, mailer := newTestService(t) // synchronous delivery
	actor, other := covPair(t, svc)

	if _, err := svc.AddExpense(ctx, covInput(actor, actor, other, 1000)); err != nil {
		t.Fatalf("AddExpense: %v", err)
	}
	if len(mailer.msgs) != 0 {
		t.Fatalf("sent %d emails to a user who never opted in, want 0", len(mailer.msgs))
	}
	// The notification itself was still recorded -- the row is not gated on mail.
	if got := covNotifications(t, svc, other.ID); len(got) != 1 {
		t.Fatalf("recorded %d notifications, want 1", len(got))
	}

	covOptInEmail(t, svc, other)
	if _, err := svc.AddExpense(ctx, covInput(actor, actor, other, 1000)); err != nil {
		t.Fatalf("AddExpense after opt-in: %v", err)
	}
	if len(mailer.msgs) != 1 {
		t.Fatalf("sent %d emails after opt-in, want 1", len(mailer.msgs))
	}
	if body := mailer.msgs[0].Body; !strings.Contains(body, "You owe") {
		t.Errorf("email body does not state the recipient's share: %q", body)
	}
}

// TestNotifyNothingDeliveredWithoutARow asserts the invariant that makes the
// table the single source of truth: if the row cannot be written, no email and
// no push go out either.
func TestNotifyNothingDeliveredWithoutARow(t *testing.T) {
	ctx := context.Background()
	svc, mailer := newTestService(t)
	actor, other := covPair(t, svc)
	covOptInEmail(t, svc, other)

	e := &store.Expense{ID: "exp-1", Name: "Dinner", Amount: 1000, Currency: "USD"}
	parts := []store.ExpenseParticipant{{UserID: other.ID, Amount: -1000}}
	// A closed handle makes the insert fail the way a real outage would.
	if err := svc.Store.DB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	svc.notifyExpense(ctx, e, parts, actor.ID, KindExpenseAdded)

	if len(mailer.msgs) != 0 {
		t.Errorf("sent %d emails for an unrecorded notification, want 0", len(mailer.msgs))
	}
}

// TestNotifyStoreFailureDoesNotFailTheWrite asserts the stated tradeoff: an
// expense the user is entitled to create still succeeds when its notification
// cannot be recorded.
func TestNotifyStoreFailureDoesNotFailTheWrite(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	actor, other := covPair(t, svc)

	e := &store.Expense{ID: "exp-1", Name: "Dinner", Amount: 1000, Currency: "USD"}
	parts := []store.ExpenseParticipant{{UserID: other.ID, Amount: -1000}}
	if err := svc.Store.DB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	// notifyExpense returns no error by design; the assertion is that it neither
	// panics nor propagates, so a caller mid-write is unaffected.
	svc.notifyExpense(ctx, e, parts, actor.ID, KindExpenseAdded)
}

// TestNotificationsForSkipsActorAndEmpty covers the row-building helper's edges
// without a store: a solo actor produces nothing to send.
func TestNotificationsForSkipsActorAndEmpty(t *testing.T) {
	e := &store.Expense{ID: "exp-1", Name: "Solo", Amount: 100, Currency: "USD"}
	if got := notificationsFor(e, nil, 1, KindExpenseAdded); len(got) != 0 {
		t.Errorf("no participants: %d rows, want 0", len(got))
	}
	only := []store.ExpenseParticipant{{UserID: 1, Amount: 0}}
	if got := notificationsFor(e, only, 1, KindExpenseAdded); len(got) != 0 {
		t.Errorf("actor-only expense: %d rows, want 0", len(got))
	}
}

// TestServiceTFallsBackToTheKey covers the nil-bundle guard: a service that
// could not load its catalogs still renders something visible.
func TestServiceTFallsBackToTheKey(t *testing.T) {
	svc, _ := newTestService(t)
	if got := svc.t("en", "notif."+KindExpenseAdded); strings.Contains(got, "notif.") {
		t.Errorf("loaded bundle should translate, got %q", got)
	}
	svc.I18n = nil
	if got := svc.t("en", "notif.expense_added"); got != "notif.expense_added" {
		t.Errorf("nil bundle: t() = %q, want the key back", got)
	}
}

// TestShareLineBranches covers all three balance directions of the email's
// share line, including the zero case no expense split normally produces.
func TestShareLineBranches(t *testing.T) {
	svc, _ := newTestService(t)
	cases := []struct {
		amount int64
		want   string
	}{
		{-500, "You owe"},
		{500, "You are owed"},
		{0, "unchanged"},
	}
	for _, c := range cases {
		got := svc.shareLine("en", covNotif(1, c.amount))
		if !strings.Contains(got, c.want) {
			t.Errorf("shareLine(%d) = %q, want it to mention %q", c.amount, got, c.want)
		}
	}
}
