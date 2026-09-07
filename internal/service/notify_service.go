package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/push"
	"github.com/hafio/gosplit/internal/store"
)

// Notification kinds. Each is both the notifications.kind value and the stem of
// an i18n key ("notif.<kind>"), so adding a kind is a constant plus a locale
// entry -- never a switch in a handler or a template.
const (
	KindExpenseAdded      = "expense_added"
	KindExpenseUpdated    = "expense_updated"
	KindExpenseDeleted    = "expense_deleted"
	KindSettlementAdded   = "settlement_added"
	KindSettlementUpdated = "settlement_updated"
	// KindSettledUpGroup is push-only: settling a whole group records a row per
	// transfer but coalesces the device toast into one (see notifySettleBatch).
	KindSettledUpGroup = "settled_up_group"
)

// entityExpense is the only entity_type in use. Settlements are expenses with
// split_type SETTLEMENT, so they link the same way.
const entityExpense = "expense"

// notifyDeliverTimeout bounds a delivery goroutine. It cannot use the request
// context -- that is already cancelled by the time it runs -- and the previous
// bare context.Background() let a hung SMTP session or push endpoint run
// unbounded. Matches the timeout sendInvite and SendMagicLink already use.
const notifyDeliverTimeout = 30 * time.Second

// unknownActor labels an actor whose account is gone. actor_id carries no
// foreign key by design, so this is a reachable state, not a corruption.
const unknownActor = "Someone"

// notifyExpense records an in-app notification for every participant except the
// actor, then delivers the same set to web push and, for recipients who opted
// in, email.
//
// The rows are written SYNCHRONOUSLY on the caller's context. They are the
// durable record and the only channel enabled by default, so a bell that misses
// an event because a goroutine never ran is a lost notification rather than a
// slow one -- and writing first makes "nothing is delivered that was not
// recorded" an invariant instead of a hope. It is one multi-row insert in a
// single transaction, negligible beside the expense write the caller just did.
//
// A failed insert is logged and does NOT fail the caller: aborting an expense
// because its notification could not be recorded would be the worse outcome.
//
// Push and email stay off the request path via s.dispatch -- unlike the bare
// goroutine this replaces, which bypassed dispatch and so could not be made
// deterministic by SetSynchronousEmail.
func (s *Service) notifyExpense(ctx context.Context, e *store.Expense,
	parts []store.ExpenseParticipant, actorID int64, kind string) {
	rows := notificationsFor(e, parts, actorID, kind)
	if len(rows) == 0 {
		return
	}
	if err := s.Store.CreateNotifications(ctx, rows); err != nil {
		slog.Warn("notify: record notifications", "kind", kind, "expense", e.ID, "err", err)
		return // never deliver what was not recorded
	}
	s.deliver(rows, actorID, "/expenses/"+e.ID, true)
}

// notifySettleBatch records the notifications for a batch of settlements but
// sends one push per recipient for the whole batch. Settling a group creates one
// settlement per transfer, and a toast for each would arrive as a burst from a
// single button press; the rows stay per-transfer so the bell and its history
// remain accurate and every entry still deep-links to its own settlement.
func (s *Service) notifySettleBatch(ctx context.Context, settled []*store.Expense,
	partsByID map[string][]store.ExpenseParticipant, actorID, groupID int64, groupName string) {
	var rows []*store.Notification
	for _, e := range settled {
		rows = append(rows, notificationsFor(e, partsByID[e.ID], actorID, KindSettlementAdded)...)
	}
	if len(rows) == 0 {
		return
	}
	if err := s.Store.CreateNotifications(ctx, rows); err != nil {
		slog.Warn("notify: record settle batch", "group", groupID, "err", err)
		return
	}
	// One toast per recipient, naming the group rather than any one transfer.
	// These are delivery-only values and are never persisted.
	coalesced := make([]*store.Notification, 0, len(rows))
	seen := map[int64]bool{}
	for _, n := range rows {
		if seen[n.UserID] {
			continue
		}
		seen[n.UserID] = true
		coalesced = append(coalesced, &store.Notification{
			UserID: n.UserID, ActorID: actorID,
			Kind: KindSettledUpGroup, Title: groupName,
		})
	}
	s.deliver(coalesced, actorID, fmt.Sprintf("/groups/%d", groupID), false)
}

// notificationsFor builds one notification per participant other than the
// actor. Amount is that recipient's own signed share, which is what lets an
// entry render "you owe ..." from the row alone.
func notificationsFor(e *store.Expense, parts []store.ExpenseParticipant,
	actorID int64, kind string) []*store.Notification {
	rows := make([]*store.Notification, 0, len(parts))
	for _, p := range parts {
		if p.UserID == actorID {
			continue
		}
		rows = append(rows, &store.Notification{
			UserID:     p.UserID,
			ActorID:    actorID,
			Kind:       kind,
			EntityType: entityExpense,
			EntityID:   e.ID,
			Title:      e.Name,
			Amount:     p.Amount,
			Currency:   e.Currency,
		})
	}
	return rows
}

// deliver pushes and (when withEmail) emails recorded notifications off the
// request path. Recipients and their languages are resolved once for the whole
// batch rather than per row.
func (s *Service) deliver(rows []*store.Notification, actorID int64, url string, withEmail bool) {
	s.sendMailAsync(func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyDeliverTimeout)
		defer cancel()

		actor := unknownActor
		if u, err := s.Store.GetUser(ctx, actorID); err == nil && u.Name != "" {
			actor = u.Name
		}
		users := make(map[int64]*store.User, len(rows))
		for _, n := range rows {
			if _, ok := users[n.UserID]; ok {
				continue
			}
			if u, err := s.Store.GetUser(ctx, n.UserID); err == nil {
				users[n.UserID] = u
			}
		}

		s.pushNotifications(ctx, rows, users, actor, url)
		if withEmail {
			s.emailNotifications(ctx, rows, users, actor, url)
		}
	})
}

// pushNotifications sends a web push per recipient, each rendered in that
// recipient's own language. Subscriptions the browser has abandoned are pruned
// as they are discovered.
func (s *Service) pushNotifications(ctx context.Context, rows []*store.Notification,
	users map[int64]*store.User, actor, url string) {
	if s.Push == nil || !s.Push.Enabled() || len(rows) == 0 {
		return
	}
	ids := make([]int64, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	subs, err := s.Store.ListPushSubscriptions(ctx, ids)
	if err != nil {
		slog.Warn("notify: list subscriptions", "err", err)
		return
	}
	byUser := map[int64][]store.PushSubscription{}
	for _, sub := range subs {
		byUser[sub.UserID] = append(byUser[sub.UserID], sub)
	}
	for _, n := range rows {
		u, ok := users[n.UserID]
		if !ok || len(byUser[n.UserID]) == 0 {
			continue
		}
		payload := push.Payload{
			Title: s.t(u.PreferredLanguage, "notif.push_title"),
			Body:  s.renderKind(u.PreferredLanguage, actor, n),
			URL:   url,
		}
		for _, sub := range byUser[n.UserID] {
			gone, err := s.Push.Send(sub.Subscription, payload)
			if gone {
				_ = s.Store.DeletePushSubscription(ctx, sub.UserID, sub.Endpoint)
			} else if err != nil {
				slog.Warn("notify: push send", "user", sub.UserID, "err", err)
			}
		}
	}
}

// emailNotifications emails the recipients who opted in. The opt-in defaults to
// off, so an account that never asked for mail never receives any.
func (s *Service) emailNotifications(ctx context.Context, rows []*store.Notification,
	users map[int64]*store.User, actor, url string) {
	for _, n := range rows {
		u, ok := users[n.UserID]
		if !ok || !u.EmailExpenseNotify || u.Email == "" {
			continue
		}
		lang := u.PreferredLanguage
		body := fmt.Sprintf("%s\n%s\n\n%s%s",
			s.renderKind(lang, actor, n), s.shareLine(lang, n), s.Config.BaseURL, url)
		if err := s.Mail.Send(ctx, u.Email, s.t(lang, "notif.email_subject"), body); err != nil {
			slog.Warn("notify: expense email failed", "to", u.Email, "err", err)
		}
	}
}

// renderKind builds the one-line description of a notification. Kind selects the
// locale string; the arguments are always (actor name, title), in that order, so
// every kind shares one call shape and cannot drift in arity.
func (s *Service) renderKind(lang, actor string, n *store.Notification) string {
	return fmt.Sprintf(s.t(lang, "notif."+n.Kind), actor, n.Title)
}

// shareLine states what the notification does to the recipient's balance.
func (s *Service) shareLine(lang string, n *store.Notification) string {
	switch {
	case n.Amount < 0:
		return fmt.Sprintf(s.t(lang, "notif.you_owe"), money.FormatWithCode(-n.Amount, n.Currency))
	case n.Amount > 0:
		return fmt.Sprintf(s.t(lang, "notif.you_are_owed"), money.FormatWithCode(n.Amount, n.Currency))
	default:
		return s.t(lang, "notif.balance_unchanged")
	}
}
