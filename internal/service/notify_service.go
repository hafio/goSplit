package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/push"
	"github.com/hafio/gosplit/internal/store"
)

// notifyExpense sends web-push + email to the expense's participants (except the
// actor) that an expense was added or updated. Best-effort and asynchronous —
// failures are logged, never surfaced to the request. Runs on a background
// context so it survives the originating request's lifecycle.
func (s *Service) notifyExpense(e *store.Expense, parts []store.ExpenseParticipant, actorID int64, verb string) {
	go func() {
		ctx := context.Background()
		recipients := make([]int64, 0, len(parts))
		amountByUser := map[int64]int64{}
		for _, p := range parts {
			amountByUser[p.UserID] = p.Amount
			if p.UserID != actorID {
				recipients = append(recipients, p.UserID)
			}
		}
		title := fmt.Sprintf("Expense %s: %s", verb, e.Name)

		s.pushToUsers(ctx, recipients, title, e)
		s.emailParticipants(ctx, recipients, amountByUser, e, verb)
	}()
}

func (s *Service) pushToUsers(ctx context.Context, userIDs []int64, title string, e *store.Expense) {
	if s.Push == nil || !s.Push.Enabled() || len(userIDs) == 0 {
		return
	}
	subs, err := s.Store.ListPushSubscriptions(ctx, userIDs)
	if err != nil {
		slog.Warn("notify: list subscriptions", "err", err)
		return
	}
	payload := push.Payload{
		Title: title,
		Body:  fmt.Sprintf("%s %s", money.FormatWithCode(e.Amount, e.Currency), e.Name),
		URL:   "/expenses/" + e.ID,
	}
	for _, sub := range subs {
		gone, err := s.Push.Send(sub.Subscription, payload)
		if gone {
			_ = s.Store.DeletePushSubscription(ctx, sub.UserID, sub.Endpoint)
		} else if err != nil {
			slog.Warn("notify: push send", "user", sub.UserID, "err", err)
		}
	}
}

func (s *Service) emailParticipants(ctx context.Context, userIDs []int64, amountByUser map[int64]int64, e *store.Expense, verb string) {
	for _, uid := range userIDs {
		u, err := s.Store.GetUser(ctx, uid)
		if err != nil || u.Email == "" {
			continue
		}
		amt := amountByUser[uid]
		var line string
		switch {
		case amt < 0:
			line = fmt.Sprintf("You owe %s.", money.FormatWithCode(-amt, e.Currency))
		case amt > 0:
			line = fmt.Sprintf("You are owed %s.", money.FormatWithCode(amt, e.Currency))
		default:
			line = "Your balance is unchanged."
		}
		body := fmt.Sprintf("An expense was %s: %q (%s).\n%s\n\n%s/expenses/%s",
			verb, e.Name, money.FormatWithCode(e.Amount, e.Currency), line, s.Config.BaseURL, e.ID)
		if err := s.Mail.Send(ctx, u.Email, "GoSplit: expense "+verb, body); err != nil {
			slog.Warn("notify: expense email failed", "to", u.Email, "err", err)
		}
	}
}
