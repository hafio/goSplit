package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

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

// generateOne creates a fresh expense from a recurrence's template, copying its
// participant rows verbatim (they are already the zero-sum stored amounts).
func (s *Service) generateOne(ctx context.Context, rec *store.ExpenseRecurrence) error {
	tmpl, err := s.Store.GetExpense(ctx, rec.TemplateExpenseID)
	if err != nil {
		return err
	}
	parts, err := s.Store.GetParticipants(ctx, tmpl.ID)
	if err != nil {
		return err
	}
	e := &store.Expense{
		Name: tmpl.Name, Category: tmpl.Category, Amount: tmpl.Amount,
		SplitType: tmpl.SplitType, ExpenseDate: todayISO(), Currency: tmpl.Currency,
		PaidBy: tmpl.PaidBy, AddedBy: rec.CreatedBy, GroupID: tmpl.GroupID,
		RecurrenceID: sql.NullInt64{Int64: rec.ID, Valid: true},
	}
	_, err = s.Store.CreateExpense(ctx, e, parts)
	return err
}

func toISO(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
