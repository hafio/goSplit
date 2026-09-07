package httpapp

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/i18n"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
)

// notificationKinds is every kind the service can write. It lives here because
// the kind strings are concatenated into i18n keys at runtime, so nothing in the
// build would catch a kind with no locale entry -- only a user seeing a raw key.
var notificationKinds = []string{
	service.KindExpenseAdded,
	service.KindExpenseUpdated,
	service.KindExpenseDeleted,
	service.KindSettlementAdded,
	service.KindSettlementUpdated,
	service.KindSettledUpGroup,
}

// seedNotification inserts one notification for userID and returns it.
func seedNotification(t *testing.T, h *harness, userID, actorID int64, entityID string) *store.Notification {
	t.Helper()
	n, err := h.st.CreateNotification(context.Background(), &store.Notification{
		UserID: userID, ActorID: actorID, Kind: "expense_added",
		EntityType: "expense", EntityID: entityID,
		Title: "Dinner", Amount: -500, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("seed notification: %v", err)
	}
	return n
}

// userByEmail looks a registered user up so a test can address their rows.
func userByEmail(t *testing.T, h *harness, email string) *store.User {
	t.Helper()
	u, err := h.st.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("get user %s: %v", email, err)
	}
	return u
}

// reread returns a notification straight from the store, bypassing the handlers.
func reread(t *testing.T, h *harness, id, userID int64) *store.Notification {
	t.Helper()
	n, err := h.st.GetNotification(context.Background(), id, userID)
	if err != nil {
		t.Fatalf("reread notification %d: %v", id, err)
	}
	return n
}

// TestNotificationsPageRendersAndGuardsForms covers the list page: the entry is
// shown, and its forms carry the double-submit guard that
// TestDoubleSubmitGuards asserts for every other mutating page (it cannot cover
// this one, because the page renders no form until a notification exists).
func TestNotificationsPageRendersAndGuardsForms(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	me := userByEmail(t, h, "alice@example.com")
	seedNotification(t, h, me.ID, me.ID, "exp-1")

	b := body(t, h.get("/notifications"))
	if !strings.Contains(b, "Dinner") {
		t.Error("notification title is not on the page")
	}
	if !strings.Contains(b, "You owe") {
		t.Error("the recipient's share is not rendered")
	}
	const guard = `hx-disabled-elt="find button[type=submit]"`
	if !strings.Contains(b, guard) {
		t.Error("a mutating form on /notifications has no double-submit guard")
	}
}

// TestNotificationsPageEmptyState covers the no-notifications branch.
func TestNotificationsPageEmptyState(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	b := body(t, h.get("/notifications"))
	if !strings.Contains(b, "No notifications yet") {
		t.Error("empty state is missing from /notifications")
	}
}

// TestNotificationsRequireLogin covers the auth boundary on every route the
// feature adds, including the two fragment endpoints.
func TestNotificationsRequireLogin(t *testing.T) {
	h := newHarness(t)
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, path := range []string{"/notifications", "/notifications/menu", "/notifications/badge"} {
		resp := h.get(path)
		_ = body(t, resp)
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("%s anonymous status = %d, want 303 to login", path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/login") {
			t.Errorf("%s anonymous redirect = %q, want /login", path, loc)
		}
	}
}

// TestNotificationBadgeAndMenuFragments covers the two blocks the topbar bell
// fetches: the badge count and the dropdown list.
func TestNotificationBadgeAndMenuFragments(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	me := userByEmail(t, h, "alice@example.com")

	if b := body(t, h.get("/notifications/badge")); strings.Contains(b, `class="badge"`) {
		t.Error("badge rendered a count with nothing unread")
	}
	seedNotification(t, h, me.ID, me.ID, "exp-1")

	b := body(t, h.get("/notifications/badge"))
	if !strings.Contains(b, `id="notif-badge"`) {
		t.Error("badge fragment is missing its swap target id")
	}
	if !strings.Contains(b, `class="badge"`) || !strings.Contains(b, ">1<") {
		t.Errorf("badge does not show the unread count: %q", b)
	}

	menu := body(t, h.get("/notifications/menu"))
	if !strings.Contains(menu, `id="notif-menu"`) {
		t.Error("menu fragment is missing its swap target id")
	}
	if !strings.Contains(menu, "Dinner") {
		t.Error("menu does not list the notification")
	}
}

// TestBellBadgeAppearsOnEveryPage covers the ViewData plumbing: the count is
// layout chrome, so it has to show up on a page that knows nothing about
// notifications.
func TestBellBadgeAppearsOnEveryPage(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	me := userByEmail(t, h, "alice@example.com")
	seedNotification(t, h, me.ID, me.ID, "exp-1")

	b := body(t, h.get("/balances"))
	if !strings.Contains(b, `class="badge"`) {
		t.Error("the bell badge does not render outside the notifications page")
	}
	if !strings.Contains(b, "#bell") {
		t.Error("the bell icon is missing from the topbar")
	}
}

// TestNotificationOpenMarksReadAndRedirects covers the click-through: the row is
// marked read and the reader lands on the expense it names.
func TestNotificationOpenMarksReadAndRedirects(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	me := userByEmail(t, h, "alice@example.com")
	n := seedNotification(t, h, me.ID, me.ID, "exp-1")

	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/notifications/"+strconv.FormatInt(n.ID, 10)+"/open", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("open status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/expenses/exp-1" {
		t.Errorf("open redirect = %q, want /expenses/exp-1", loc)
	}
	if !reread(t, h, n.ID, me.ID).IsRead() {
		t.Error("opening a notification did not mark it read")
	}
}

// TestNotificationOpenUnknownIDFallsBack covers both non-numeric and
// non-existent ids: neither errors, both land on the list page.
func TestNotificationOpenUnknownIDFallsBack(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, id := range []string{"not-a-number", "999999"} {
		resp := h.post("/notifications/"+id+"/open", url.Values{})
		_ = body(t, resp)
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("open %q status = %d, want 303", id, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/notifications" {
			t.Errorf("open %q redirect = %q, want /notifications", id, loc)
		}
	}
}

// TestNotificationOpenIsScopedToItsOwner is the request-level half of the
// cross-tenant guard the store enforces in SQL. Bob must not be able to mark
// Alice's notification read, and the response must not reveal that it exists.
func TestNotificationOpenIsScopedToItsOwner(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	alice := userByEmail(t, h, "alice@example.com")
	n := seedNotification(t, h, alice.ID, alice.ID, "exp-1")

	h.register("Bob", "bob@example.com", "password123")
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/notifications/"+strconv.FormatInt(n.ID, 10)+"/open", url.Values{})
	_ = body(t, resp)
	// Same answer as a missing id: no 403, no leak of the expense it points at.
	if loc := resp.Header.Get("Location"); loc != "/notifications" {
		t.Errorf("cross-user open redirect = %q, want /notifications (no leak)", loc)
	}
	if reread(t, h, n.ID, alice.ID).IsRead() {
		t.Error("another user marked Alice's notification read")
	}
}

// TestNotificationsReadAllIsScopedToItsOwner covers mark-all plus its isolation.
func TestNotificationsReadAllIsScopedToItsOwner(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	alice := userByEmail(t, h, "alice@example.com")
	mine := seedNotification(t, h, alice.ID, alice.ID, "exp-1")

	h.register("Bob", "bob@example.com", "password123")
	bob := userByEmail(t, h, "bob@example.com")
	theirs := seedNotification(t, h, bob.ID, alice.ID, "exp-2")

	// Bob marks everything read: only his own row changes.
	_ = body(t, h.post("/notifications/read-all", url.Values{}))
	if !reread(t, h, theirs.ID, bob.ID).IsRead() {
		t.Error("mark-all did not mark the caller's own notification read")
	}
	if reread(t, h, mine.ID, alice.ID).IsRead() {
		t.Error("mark-all reached another user's notifications")
	}
	if b := body(t, h.get("/notifications/badge")); strings.Contains(b, `class="badge"`) {
		t.Error("badge still shows a count after mark-all")
	}
}

// TestNotificationOpenRejectsMissingCSRF asserts the mutation is refused AND the
// row is left alone -- a rejected request must not reach the store at all.
func TestNotificationOpenRejectsMissingCSRF(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	me := userByEmail(t, h, "alice@example.com")
	n := seedNotification(t, h, me.ID, me.ID, "exp-1")

	// PostForm without the token: h.post would add it, so post directly.
	resp, err := h.client.PostForm(h.srv.URL+"/notifications/"+strconv.FormatInt(n.ID, 10)+"/open", url.Values{})
	if err != nil {
		t.Fatalf("post without csrf: %v", err)
	}
	_ = body(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	if reread(t, h, n.ID, me.ID).IsRead() {
		t.Error("a CSRF-rejected request still marked the notification read")
	}
}

// TestNotificationEntityHrefFallback covers the unknown-entity branch directly:
// the type is stored, so a row written by a future version must still be
// clickable rather than pointing nowhere.
func TestNotificationEntityHrefFallback(t *testing.T) {
	cases := map[string]string{
		"expense": "/expenses/x",
		"group":   "/notifications",
		"":        "/notifications",
	}
	for entityType, want := range cases {
		got := entityHref(&store.Notification{EntityType: entityType, EntityID: "x"})
		if got != want {
			t.Errorf("entityHref(%q) = %q, want %q", entityType, got, want)
		}
	}
}

// TestNotificationKindsHaveLocaleEntries guards the one thing the template key
// checker cannot see: kind strings are concatenated into i18n keys at runtime,
// so a new kind with no locale entry would ship silently and render the raw key.
func TestNotificationKindsHaveLocaleEntries(t *testing.T) {
	bundle, err := i18n.Load()
	if err != nil {
		t.Fatalf("load i18n: %v", err)
	}
	for _, kind := range notificationKinds {
		key := "notif." + kind
		got := bundle.T(i18n.DefaultLang, key)
		if got == key {
			t.Errorf("kind %q has no %q entry in the default locale", kind, key)
		}
		// Every kind's message takes (actor, title), so both verbs must be there.
		if strings.Count(got, "%s") != 2 {
			t.Errorf("%q = %q, want exactly two %%s (actor, title)", key, got)
		}
	}
}

// TestProfileEmailOptInTogglesBothWays is the regression guard for the
// absent-when-unchecked checkbox: the surrounding handler uses "only if present"
// for its text fields, which would make this impossible to switch back off.
func TestProfileEmailOptInTogglesBothWays(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	if userByEmail(t, h, "alice@example.com").EmailExpenseNotify {
		t.Fatal("the email opt-in must be off for a new account")
	}
	_ = body(t, h.post("/profile", url.Values{
		"name": {"Alice"}, "email_expense_notify": {"1"},
	}))
	if !userByEmail(t, h, "alice@example.com").EmailExpenseNotify {
		t.Fatal("checking the box did not enable the email opt-in")
	}
	// Submitting without the field is what an unchecked box actually sends.
	_ = body(t, h.post("/profile", url.Values{"name": {"Alice"}}))
	if userByEmail(t, h, "alice@example.com").EmailExpenseNotify {
		t.Error("unchecking the box did not disable the email opt-in")
	}
}
