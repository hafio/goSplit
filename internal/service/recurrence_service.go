package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field cron expressions (min hour dom mon dow).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CreateRecurrence validates the cron expression and schedules generation of
// expenses from an existing template expense.
func (s *Service) CreateRecurrence(ctx context.Context, actor int64, templateExpenseID, cronExpr string) (*store.ExpenseRecurrence, error) {
	sched, err := cronParser.Parse(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	tmpl, err := s.Store.GetExpense(ctx, templateExpenseID)
	if err != nil {
		return nil, err
	}
	if tmpl.SplitType == "CURRENCY_CONVERSION" || tmpl.ConversionToID.Valid {
		return nil, errors.New("currency-conversion expenses cannot recur")
	}
	next := sched.Next(time.Now().UTC())
	jobName := fmt.Sprintf("rec-%s-%d", templateExpenseID, time.Now().UTC().UnixNano())
	return s.Store.CreateRecurrence(ctx, &store.ExpenseRecurrence{
		CronExpression:    cronExpr,
		JobName:           jobName,
		TemplateExpenseID: templateExpenseID,
		CreatedBy:         actor,
		NextRunAt:         sql.NullString{String: toISO(next), Valid: true},
	})
}

// ListRecurrences returns a user's recurrences.
func (s *Service) ListRecurrences(ctx context.Context, userID int64) ([]*store.ExpenseRecurrence, error) {
	return s.Store.ListRecurrencesForUser(ctx, userID)
}

// DeleteRecurrence removes a user's recurrence.
func (s *Service) DeleteRecurrence(ctx context.Context, id, userID int64) error {
	return s.Store.DeleteRecurrence(ctx, id, userID)
}

// GenerateDueRecurrences produces expenses for every due recurrence and
// advances each to its next fire time. Called by the scheduler under the leader
// lock, so generation happens once. Returns the number generated.
func (s *Service) GenerateDueRecurrences(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	due, err := s.Store.ListDueRecurrences(ctx, toISO(now))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, rec := range due {
		sched, err := cronParser.Parse(rec.CronExpression)
		if err != nil {
			slog.Warn("recurrence: bad cron, skipping", "id", rec.ID, "err", err)
			continue
		}
		if err := s.generateOne(ctx, rec); err != nil {
			slog.Warn("recurrence: generate failed", "id", rec.ID, "err", err)
			// Still advance to avoid a tight retry loop on a broken template.
		} else {
			count++
		}
		if err := s.Store.AdvanceRecurrence(ctx, rec.ID, toISO(sched.Next(now))); err != nil {
			return count, err
		}
	}
	return count, nil
}

// generateOne creates a fresh expense from a recurrence's template.
//
// When the template records the raw split inputs, the generated expense is
// re-split from them for its own date and carries the inputs forward, so it is
// editable with the same fidelity as the template. Recomputing (rather than
// copying the amounts) is what keeps the two consistent: the leftover minor
// units are distributed from a seed that seeds on the expense date, so replayed
// inputs and copied amounts could otherwise disagree by a unit and a no-op edit
// would move a cent.
//
// Templates with no recorded inputs -- anything created before they were stored
// -- keep the original behaviour and copy the participant rows verbatim (they
// are already the zero-sum stored amounts).
func (s *Service) generateOne(ctx context.Context, rec *store.ExpenseRecurrence) error {
	tmpl, err := s.Store.GetExpense(ctx, rec.TemplateExpenseID)
	if err != nil {
		return err
	}
	parts, err := s.Store.GetParticipants(ctx, tmpl.ID)
	if err != nil {
		return err
	}
	date := todayISO()
	method := split.Method(tmpl.SplitType)
	payload, _ := s.Store.GetSplitInputs(ctx, tmpl.ID)
	values, ok := split.DecodeInputs(payload, method)
	if ok {
		lines := make([]split.Line, 0, len(values))
		for _, p := range parts {
			if v, in := values[p.UserID]; in {
				lines = append(lines, split.LineFromInput(method, p.UserID, v))
			}
		}
		recomputed, cErr := split.Compute(method, tmpl.Amount, tmpl.PaidBy, date, lines)
		if cErr != nil {
			// The template's inputs no longer produce a valid split (a
			// participant left, say). Fall back to copying its rows rather than
			// skipping the occurrence entirely.
			ok = false
		} else {
			parts = make([]store.ExpenseParticipant, len(recomputed))
			for i, p := range recomputed {
				parts[i] = store.ExpenseParticipant{UserID: p.UserID, Amount: p.Amount}
			}
		}
	}
	e := &store.Expense{
		Name: tmpl.Name, Category: tmpl.Category, Amount: tmpl.Amount,
		SplitType: tmpl.SplitType, ExpenseDate: date, Currency: tmpl.Currency,
		PaidBy: tmpl.PaidBy, AddedBy: rec.CreatedBy, GroupID: tmpl.GroupID,
		RecurrenceID: sql.NullInt64{Int64: rec.ID, Valid: true},
	}
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil {
		return err
	}
	if ok {
		_ = s.Store.PutSplitInputs(ctx, created.ID, payload)
	}
	return nil
}

func toISO(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
