package store

import (
	"context"
	"database/sql"
	"errors"
)

const recurrenceCols = `id, cron_expression, job_name, template_expense_id, notified,
	created_by, created_at, next_run_at`

func scanRecurrence(row interface{ Scan(...any) error }) (*ExpenseRecurrence, error) {
	var r ExpenseRecurrence
	err := row.Scan(&r.ID, &r.CronExpression, &r.JobName, &r.TemplateExpenseID,
		&r.Notified, &r.CreatedBy, &r.CreatedAt, &r.NextRunAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateRecurrence inserts a recurrence and returns it with its id.
func (s *Store) CreateRecurrence(ctx context.Context, r *ExpenseRecurrence) (*ExpenseRecurrence, error) {
	id, err := s.insertReturningID(ctx,
		`INSERT INTO expense_recurrences (cron_expression, job_name, template_expense_id, created_by, next_run_at)
		 VALUES (?, ?, ?, ?, ?)`, "expense_recurrences",
		r.CronExpression, r.JobName, r.TemplateExpenseID, r.CreatedBy, r.NextRunAt)
	if err != nil {
		return nil, err
	}
	return s.GetRecurrence(ctx, id)
}

// GetRecurrence returns a recurrence by id.
func (s *Store) GetRecurrence(ctx context.Context, id int64) (*ExpenseRecurrence, error) {
	return scanRecurrence(s.DB.QueryRowContext(ctx, s.rebind(
		`SELECT `+recurrenceCols+` FROM expense_recurrences WHERE id = ?`), id))
}

// ListRecurrencesForUser lists recurrences created by a user.
func (s *Store) ListRecurrencesForUser(ctx context.Context, userID int64) ([]*ExpenseRecurrence, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT `+recurrenceCols+` FROM expense_recurrences WHERE created_by = ? ORDER BY id DESC`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ExpenseRecurrence
	for rows.Next() {
		r, err := scanRecurrence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRecurrence removes a recurrence (only if owned by userID).
func (s *Store) DeleteRecurrence(ctx context.Context, id, userID int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`DELETE FROM expense_recurrences WHERE id = ? AND created_by = ?`), id, userID)
	return err
}

// ListDueRecurrences returns recurrences whose next_run_at is at or before now.
func (s *Store) ListDueRecurrences(ctx context.Context, nowIso string) ([]*ExpenseRecurrence, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT `+recurrenceCols+` FROM expense_recurrences
		 WHERE next_run_at IS NOT NULL AND next_run_at <= ? ORDER BY id`), nowIso)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ExpenseRecurrence
	for rows.Next() {
		r, err := scanRecurrence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdvanceRecurrence sets the next fire time for a recurrence.
func (s *Store) AdvanceRecurrence(ctx context.Context, id int64, nextIso string) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE expense_recurrences SET next_run_at = ? WHERE id = ?`), nextIso, id)
	return err
}
