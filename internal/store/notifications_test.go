package store

import (
	"context"
	"errors"
	"testing"
)

// covNotification inserts a notification for userID, failing the test on error.
func covNotification(t *testing.T, st *Store, userID, actorID int64, entityID string) *Notification {
	t.Helper()
	n, err := st.CreateNotification(context.Background(), &Notification{
		UserID: userID, ActorID: actorID, Kind: "expense_added",
		EntityType: "expense", EntityID: entityID,
		Title: "Dinner", Amount: -500, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("create notification for %d: %v", userID, err)
	}
	return n
}

// TestNotificationRoundTrip covers create, read back, list order and the unread
// count, plus the ErrNotFound miss.
func TestNotificationRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")

	first := covNotification(t, st, a.ID, a.ID, "exp-1")
	if first.ID == 0 {
		t.Fatal("create did not assign an id")
	}
	if first.IsRead() {
		t.Error("a new notification must be unread")
	}
	if first.Title != "Dinner" || first.Amount != -500 || first.Currency != "USD" {
		t.Errorf("fields did not round-trip: %+v", first)
	}
	if first.CreatedAt == "" {
		t.Error("created_at was not stamped")
	}
	second := covNotification(t, st, a.ID, a.ID, "exp-2")

	got, err := st.ListNotifications(ctx, a.ID, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("listed %d, want 2", len(got))
	}
	if got[0].ID != second.ID {
		t.Errorf("list is not newest-first: got id %d first, want %d", got[0].ID, second.ID)
	}

	limited, err := st.ListNotifications(ctx, a.ID, 1)
	if err != nil {
		t.Fatalf("list with limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit 1 returned %d rows", len(limited))
	}

	if n, err := st.CountUnreadNotifications(ctx, a.ID); err != nil || n != 2 {
		t.Errorf("unread count = %d (err %v), want 2", n, err)
	}
	if _, err := st.GetNotification(ctx, 999999, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing notification: err = %v, want ErrNotFound", err)
	}
}

// TestNotificationMarkRead covers marking one and marking all, and that a second
// mark of an already-read row leaves the original timestamp alone.
func TestNotificationMarkRead(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")
	one := covNotification(t, st, a.ID, a.ID, "exp-1")
	two := covNotification(t, st, a.ID, a.ID, "exp-2")

	if err := st.MarkNotificationRead(ctx, one.ID, a.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	read, err := st.GetNotification(ctx, one.ID, a.ID)
	if err != nil {
		t.Fatalf("get after mark: %v", err)
	}
	if !read.IsRead() {
		t.Fatal("row is still unread after MarkNotificationRead")
	}
	stamp := read.ReadAt.String

	// Marking again must not move the timestamp: the guard is read_at IS NULL.
	if err := st.MarkNotificationRead(ctx, one.ID, a.ID); err != nil {
		t.Fatalf("second mark read: %v", err)
	}
	again, err := st.GetNotification(ctx, one.ID, a.ID)
	if err != nil {
		t.Fatalf("get after second mark: %v", err)
	}
	if again.ReadAt.String != stamp {
		t.Errorf("read_at moved on a second mark: %q -> %q", stamp, again.ReadAt.String)
	}

	if n, _ := st.CountUnreadNotifications(ctx, a.ID); n != 1 {
		t.Errorf("unread count = %d, want 1", n)
	}
	if err := st.MarkAllNotificationsRead(ctx, a.ID); err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if n, _ := st.CountUnreadNotifications(ctx, a.ID); n != 0 {
		t.Errorf("unread count after mark-all = %d, want 0", n)
	}
	if last, _ := st.GetNotification(ctx, two.ID, a.ID); !last.IsRead() {
		t.Error("mark-all left a row unread")
	}
}

// TestNotificationIsScopedToItsOwner is the security boundary: userID is part of
// every WHERE clause, so one user can neither read, count nor mark another's
// notifications, and a guessed id is indistinguishable from a missing one.
func TestNotificationIsScopedToItsOwner(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")
	b := covUser(t, st, "b")
	mine := covNotification(t, st, a.ID, b.ID, "exp-1")

	// B cannot read A's notification, and learns nothing about whether it exists.
	if _, err := st.GetNotification(ctx, mine.ID, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user get: err = %v, want ErrNotFound", err)
	}
	// B cannot mark it read. The call succeeds (matching zero rows) rather than
	// erroring, because an error would itself confirm the row is there.
	if err := st.MarkNotificationRead(ctx, mine.ID, b.ID); err != nil {
		t.Fatalf("cross-user mark: %v", err)
	}
	still, err := st.GetNotification(ctx, mine.ID, a.ID)
	if err != nil {
		t.Fatalf("get own notification: %v", err)
	}
	if still.IsRead() {
		t.Error("another user marked this notification read")
	}
	// Nor by marking everything read as themselves.
	if err := st.MarkAllNotificationsRead(ctx, b.ID); err != nil {
		t.Fatalf("mark all as b: %v", err)
	}
	if got, _ := st.GetNotification(ctx, mine.ID, a.ID); got.IsRead() {
		t.Error("another user's mark-all cleared this notification")
	}
	// B's own counts and lists never include A's rows.
	if n, _ := st.CountUnreadNotifications(ctx, b.ID); n != 0 {
		t.Errorf("b unread count = %d, want 0", n)
	}
	if got, _ := st.ListNotifications(ctx, b.ID, 0); len(got) != 0 {
		t.Errorf("b listed %d of a's notifications, want 0", len(got))
	}
	if n, _ := st.CountUnreadNotifications(ctx, a.ID); n != 1 {
		t.Errorf("a unread count = %d, want 1", n)
	}
}

// TestNotificationOutlivesItsSubject proves the deliberate absence of a foreign
// key on entity_id: deleting the expense a notification names -- softly or for
// real -- neither fails nor takes the notification with it.
func TestNotificationOutlivesItsSubject(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")
	b := covUser(t, st, "b")
	e := covDirectExpense(t, st, "Dinner", "2026-03-01", a.ID, []ExpenseParticipant{
		{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500},
	})
	n := covNotification(t, st, b.ID, a.ID, e.ID)

	if err := st.SoftDeleteExpense(ctx, e.ID, a.ID, e.Version); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if got, err := st.GetNotification(ctx, n.ID, b.ID); err != nil || got.EntityID != e.ID {
		t.Fatalf("notification lost after soft delete: %+v (err %v)", got, err)
	}
	// A hard delete is what a restore does. It must not be blocked by, and must
	// not cascade to, the notification.
	if _, err := st.DB.ExecContext(ctx, st.rebind(`DELETE FROM expense_participants WHERE expense_id = ?`), e.ID); err != nil {
		t.Fatalf("delete participants: %v", err)
	}
	if _, err := st.DB.ExecContext(ctx, st.rebind(`DELETE FROM expenses WHERE id = ?`), e.ID); err != nil {
		t.Fatalf("hard delete expense: %v", err)
	}
	got, err := st.GetNotification(ctx, n.ID, b.ID)
	if err != nil {
		t.Fatalf("notification gone after hard delete: %v", err)
	}
	if got.EntityID != e.ID {
		t.Errorf("entity_id = %q, want the deleted expense's id %q", got.EntityID, e.ID)
	}
}

// TestNotificationCascadesWithItsOwner covers the one foreign key the table does
// have: a deleted account takes its notifications with it.
func TestNotificationCascadesWithItsOwner(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")
	b := covUser(t, st, "b")
	covNotification(t, st, b.ID, a.ID, "exp-1")

	if err := st.DeleteUser(ctx, b.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if got, _ := st.ListNotifications(ctx, b.ID, 0); len(got) != 0 {
		t.Errorf("deleted user still has %d notifications", len(got))
	}
}

// TestCreateNotificationsBatch covers the fan-out insert, including the empty
// case that must not open a transaction.
func TestCreateNotificationsBatch(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")
	b := covUser(t, st, "b")

	if err := st.CreateNotifications(ctx, nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}
	rows := []*Notification{
		{UserID: a.ID, ActorID: b.ID, Kind: "expense_added", EntityType: "expense",
			EntityID: "exp-1", Title: "Dinner", Amount: -500, Currency: "USD"},
		{UserID: b.ID, ActorID: a.ID, Kind: "expense_added", EntityType: "expense",
			EntityID: "exp-1", Title: "Dinner", Amount: 500, Currency: "USD"},
	}
	if err := st.CreateNotifications(ctx, rows); err != nil {
		t.Fatalf("batch insert: %v", err)
	}
	for _, u := range []int64{a.ID, b.ID} {
		if n, _ := st.CountUnreadNotifications(ctx, u); n != 1 {
			t.Errorf("user %d unread = %d, want 1", u, n)
		}
	}
}

// TestDeleteNotificationsBefore covers the retention purge: read rows go on the
// short cutoff, unread ones only on the long one, and anything recent stays.
func TestDeleteNotificationsBefore(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	a := covUser(t, st, "a")

	oldRead := covNotification(t, st, a.ID, a.ID, "old-read")
	oldUnread := covNotification(t, st, a.ID, a.ID, "old-unread")
	recent := covNotification(t, st, a.ID, a.ID, "recent")

	// Backdate two rows and mark one of them read.
	for _, id := range []int64{oldRead.ID, oldUnread.ID} {
		if _, err := st.DB.ExecContext(ctx, st.rebind(
			`UPDATE notifications SET created_at = ? WHERE id = ?`),
			"2020-01-01T00:00:00.000Z", id); err != nil {
			t.Fatalf("backdate %d: %v", id, err)
		}
	}
	if _, err := st.DB.ExecContext(ctx, st.rebind(
		`UPDATE notifications SET read_at = ? WHERE id = ?`),
		"2020-01-02T00:00:00.000Z", oldRead.ID); err != nil {
		t.Fatalf("mark old row read: %v", err)
	}

	// A read cutoff that catches the backdated rows, and an unread cutoff older
	// than all of them: only the read one goes.
	if err := st.DeleteNotificationsBefore(ctx,
		"2021-01-01T00:00:00.000Z", "2010-01-01T00:00:00.000Z"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := st.GetNotification(ctx, oldRead.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("old read notification survived the purge: err = %v", err)
	}
	if _, err := st.GetNotification(ctx, oldUnread.ID, a.ID); err != nil {
		t.Errorf("old UNREAD notification was purged on the read cutoff: %v", err)
	}

	// Now let the unread cutoff catch it too, while the recent row stays.
	if err := st.DeleteNotificationsBefore(ctx,
		"2021-01-01T00:00:00.000Z", "2021-01-01T00:00:00.000Z"); err != nil {
		t.Fatalf("second purge: %v", err)
	}
	if _, err := st.GetNotification(ctx, oldUnread.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("old unread notification survived the unread cutoff: err = %v", err)
	}
	if _, err := st.GetNotification(ctx, recent.ID, a.ID); err != nil {
		t.Errorf("recent notification was purged: %v", err)
	}
}

// TestNotificationDBErrors covers the error return of every method against a
// closed handle, so no failure path is silently untested.
func TestNotificationDBErrors(t *testing.T) {
	ctx := context.Background()
	st := cov2ClosedStore(t)
	n := &Notification{UserID: 1, ActorID: 2, Kind: "expense_added",
		EntityType: "expense", EntityID: "exp-1", Title: "X", Currency: "USD"}

	if _, err := st.CreateNotification(ctx, n); err == nil {
		t.Error("CreateNotification on a closed store should fail")
	}
	if err := st.CreateNotifications(ctx, []*Notification{n}); err == nil {
		t.Error("CreateNotifications on a closed store should fail")
	}
	if _, err := st.GetNotification(ctx, 1, 1); err == nil {
		t.Error("GetNotification on a closed store should fail")
	}
	if _, err := st.ListNotifications(ctx, 1, 10); err == nil {
		t.Error("ListNotifications on a closed store should fail")
	}
	if _, err := st.CountUnreadNotifications(ctx, 1); err == nil {
		t.Error("CountUnreadNotifications on a closed store should fail")
	}
	if err := st.MarkNotificationRead(ctx, 1, 1); err == nil {
		t.Error("MarkNotificationRead on a closed store should fail")
	}
	if err := st.MarkAllNotificationsRead(ctx, 1); err == nil {
		t.Error("MarkAllNotificationsRead on a closed store should fail")
	}
	if err := st.DeleteNotificationsBefore(ctx, "2020-01-01", "2020-01-01"); err == nil {
		t.Error("DeleteNotificationsBefore on a closed store should fail")
	}
}
