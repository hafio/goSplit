package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const expenseCols = `id, name, category, amount, split_type, expense_date, currency,
	paid_by, added_by, updated_by, group_id, file_key, transaction_id, recurrence_id,
	conversion_to_id, moved_from_id, deleted_at, deleted_by, created_at, updated_at, note`

func scanExpense(row interface{ Scan(...any) error }) (*Expense, error) {
	var e Expense
	err := row.Scan(&e.ID, &e.Name, &e.Category, &e.Amount, &e.SplitType, &e.ExpenseDate,
		&e.Currency, &e.PaidBy, &e.AddedBy, &e.UpdatedBy, &e.GroupID, &e.FileKey,
		&e.TransactionID, &e.RecurrenceID, &e.ConversionToID, &e.MovedFromID,
		&e.DeletedAt, &e.DeletedBy, &e.CreatedAt, &e.UpdatedAt, &e.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GroupScope selects group vs non-group rows for filtering (spec §5.2).
type GroupScope int

const (
	ScopeAll GroupScope = iota
	ScopeOnlyGroup
	ScopeOnlyNonGroup
)

// ExpenseFilter carries the shared §5.2 filter parameters. Zero value = no
// filtering. Filters combine with AND and never affect computed balances.
type ExpenseFilter struct {
	AmountMin *int64
	AmountMax *int64
	DateFrom  string // inclusive ISO date, "" = none
	DateTo    string // inclusive ISO date, "" = none
	Descr     string // raw user text; `*` = wildcard, plain = substring
	Scope     GroupScope
	GroupID   *int64 // specific-group selector (activity feed)
}

// apply appends this filter's conditions (using `?` placeholders) to a WHERE
// clause builder.
func (f ExpenseFilter) apply(conds *[]string, args *[]any) {
	if f.AmountMin != nil {
		*conds = append(*conds, "e.amount >= ?")
		*args = append(*args, *f.AmountMin)
	}
	if f.AmountMax != nil {
		*conds = append(*conds, "e.amount <= ?")
		*args = append(*args, *f.AmountMax)
	}
	if f.DateFrom != "" {
		*conds = append(*conds, "e.expense_date >= ?")
		*args = append(*args, f.DateFrom)
	}
	if f.DateTo != "" {
		// Inclusive upper bound: match any datetime within the day.
		*conds = append(*conds, "e.expense_date <= ?")
		*args = append(*args, f.DateTo+"T23:59:59.999Z")
	}
	if p := BuildLikePattern(f.Descr); p != "" {
		*conds = append(*conds, "lower(e.name) LIKE lower(?)")
		*args = append(*args, p)
	}
	switch f.Scope {
	case ScopeOnlyGroup:
		*conds = append(*conds, "e.group_id IS NOT NULL")
	case ScopeOnlyNonGroup:
		*conds = append(*conds, "e.group_id IS NULL")
	}
	if f.GroupID != nil {
		*conds = append(*conds, "e.group_id = ?")
		*args = append(*args, *f.GroupID)
	}
}

// BuildLikePattern converts user filter text into a SQL LIKE pattern that
// behaves identically on SQLite and Postgres: `%` and `_` in the input are
// escaped, `*` becomes `%` (wildcard), and plain text is wrapped in `%...%` to
// match as a substring. Returns "" when the input is empty.
func BuildLikePattern(descr string) string {
	descr = strings.TrimSpace(descr)
	if descr == "" {
		return ""
	}
	var b strings.Builder
	hasStar := strings.Contains(descr, "*")
	for _, r := range descr {
		switch r {
		case '%', '_', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '*':
			b.WriteByte('%')
		default:
			b.WriteRune(r)
		}
	}
	pat := b.String()
	if !hasStar {
		pat = "%" + pat + "%"
	}
	return pat
}

// CreateExpense inserts an expense and its participant rows in one transaction.
// The caller is responsible for a zero-sum participant set (spec §4.1).
func (s *Store) CreateExpense(ctx context.Context, e *Expense, parts []ExpenseParticipant) (*Expense, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.insertExpenseTx(ctx, tx, e, parts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetExpense(ctx, e.ID)
}

func (s *Store) insertExpenseTx(ctx context.Context, tx *sql.Tx, e *Expense, parts []ExpenseParticipant) error {
	if e.ID == "" {
		e.ID = NewUUID()
	}
	if e.Category == "" {
		e.Category = "general"
	}
	_, err := tx.ExecContext(ctx, s.rebind(
		`INSERT INTO expenses (id, name, category, amount, split_type, expense_date, currency,
			paid_by, added_by, updated_by, group_id, file_key, transaction_id, recurrence_id,
			conversion_to_id, moved_from_id, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		e.ID, e.Name, e.Category, e.Amount, e.SplitType, e.ExpenseDate, e.Currency,
		e.PaidBy, e.AddedBy, e.UpdatedBy, e.GroupID, e.FileKey, e.TransactionID,
		e.RecurrenceID, e.ConversionToID, e.MovedFromID, e.Note)
	if err != nil {
		return err
	}
	for _, p := range parts {
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?, ?, ?)`),
			e.ID, p.UserID, p.Amount); err != nil {
			return err
		}
	}
	return nil
}

// GetExpense returns an expense by id (including soft-deleted).
func (s *Store) GetExpense(ctx context.Context, id string) (*Expense, error) {
	return scanExpense(s.DB.QueryRowContext(ctx, s.rebind(`SELECT `+expenseCols+` FROM expenses WHERE id = ?`), id))
}

// GetParticipants returns the participant rows of an expense.
func (s *Store) GetParticipants(ctx context.Context, expenseID string) ([]ExpenseParticipant, error) {
	rows, err := s.DB.QueryContext(ctx, s.rebind(
		`SELECT expense_id, user_id, amount FROM expense_participants WHERE expense_id = ? ORDER BY user_id`), expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpenseParticipant
	for rows.Next() {
		var p ExpenseParticipant
		if err := rows.Scan(&p.ExpenseID, &p.UserID, &p.Amount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UserNetByExpense returns the viewer's signed participant amount for each of
// the given expense ids (positive = you lent, negative = you borrowed).
// Expenses the user does not participate in are absent from the map. Because
// participant rows are stored zero-sum, this value is already the net share.
func (s *Store) UserNetByExpense(ctx context.Context, userID int64, ids []string) (map[string]int64, error) {
	out := make(map[string]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, userID)
	for i, id := range ids {
		ph[i] = "?"
		args = append(args, id)
	}
	q := `SELECT expense_id, amount FROM expense_participants
		WHERE user_id = ? AND expense_id IN (` + strings.Join(ph, ",") + `)`
	rows, err := s.DB.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var amt int64
		if err := rows.Scan(&id, &amt); err != nil {
			return nil, err
		}
		out[id] = amt
	}
	return out, rows.Err()
}

// SoftDeleteExpense marks an expense deleted by a user (balances recompute
// automatically because the view excludes deleted rows).
func (s *Store) SoftDeleteExpense(ctx context.Context, id string, byUser int64) error {
	_, err := s.DB.ExecContext(ctx, s.rebind(
		`UPDATE expenses SET deleted_at = ?, deleted_by = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`),
		nowISO(), byUser, nowISO(), id)
	return err
}

// UpdateExpense replaces an expense's editable fields (including the free-text
// note), and its participant set, in one tx (edit flow). The participant set
// must remain zero-sum. created_at/added_by are left untouched.
func (s *Store) UpdateExpense(ctx context.Context, e *Expense, parts []ExpenseParticipant) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, s.rebind(
		`UPDATE expenses SET name=?, category=?, amount=?, split_type=?, expense_date=?, currency=?,
			paid_by=?, updated_by=?, group_id=?, file_key=?, note=?, updated_at=? WHERE id=?`),
		e.Name, e.Category, e.Amount, e.SplitType, e.ExpenseDate, e.Currency, e.PaidBy,
		e.UpdatedBy, e.GroupID, e.FileKey, e.Note, nowISO(), e.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM expense_participants WHERE expense_id = ?`), e.ID); err != nil {
		return err
	}
	for _, p := range parts {
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO expense_participants (expense_id, user_id, amount) VALUES (?, ?, ?)`),
			e.ID, p.UserID, p.Amount); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CreateConversionPair inserts a linked CURRENCY_CONVERSION pair in one tx: the
// "to" expense first (to obtain its id), then the "from" expense with
// conversion_to_id pointing at it (spec §5.1).
func (s *Store) CreateConversionPair(ctx context.Context, from *Expense, fromParts []ExpenseParticipant, to *Expense, toParts []ExpenseParticipant) (fromID, toID string, err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	if to.ID == "" {
		to.ID = NewUUID()
	}
	if err := s.insertExpenseTx(ctx, tx, to, toParts); err != nil {
		return "", "", err
	}
	from.ConversionToID = sql.NullString{String: to.ID, Valid: true}
	if err := s.insertExpenseTx(ctx, tx, from, fromParts); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return from.ID, to.ID, nil
}

// ListFriendExpenses returns every expense that affects the me↔friend balance,
// newest first, with the §5.2 filters applied. This includes group expenses (not
// just direct ones): an expense affects the pair exactly when one of them paid
// and the other participated — the same pairing balance_view uses — so the list
// matches the balance shown, which sums direct + all shared groups.
func (s *Store) ListFriendExpenses(ctx context.Context, me, friend int64, f ExpenseFilter) ([]*Expense, error) {
	conds := []string{
		"e.deleted_at IS NULL",
		`((e.paid_by = ? AND e.id IN (SELECT expense_id FROM expense_participants WHERE user_id = ?))
		  OR (e.paid_by = ? AND e.id IN (SELECT expense_id FROM expense_participants WHERE user_id = ?)))`,
	}
	args := []any{me, friend, friend, me}
	// The friend view ignores caller scope/group selectors.
	f.Scope = ScopeAll
	f.GroupID = nil
	f.apply(&conds, &args)
	return s.queryExpenses(ctx, conds, args)
}

// ListGroupExpenses returns a group's expenses, newest first, with §5.2 filters.
func (s *Store) ListGroupExpenses(ctx context.Context, groupID int64, f ExpenseFilter) ([]*Expense, error) {
	conds := []string{"e.deleted_at IS NULL", "e.group_id = ?"}
	args := []any{groupID}
	f.Scope = ScopeAll
	f.GroupID = nil
	f.apply(&conds, &args)
	return s.queryExpenses(ctx, conds, args)
}

// ListActivity returns all expenses (incl. soft-deleted, for the activity feed)
// that involve the user, newest first, with §5.2 filters (group scope + a
// specific-group selector).
func (s *Store) ListActivity(ctx context.Context, userID int64, f ExpenseFilter) ([]*Expense, error) {
	conds := []string{
		`(e.paid_by = ? OR e.id IN (SELECT expense_id FROM expense_participants WHERE user_id = ?)
		  OR e.group_id IN (SELECT group_id FROM group_users WHERE user_id = ?))`,
	}
	args := []any{userID, userID, userID}
	f.apply(&conds, &args)
	return s.queryExpenses(ctx, conds, args)
}

// notCollapsible excludes expenses that must stay live: currency-conversion
// pairs (linked) and recurrence templates (FK'd by expense_recurrences).
const notCollapsible = `e.split_type <> 'CURRENCY_CONVERSION'
	AND e.id NOT IN (SELECT template_expense_id FROM expense_recurrences)`

// ListDirectCollapsible returns the non-group expenses shared by exactly the two
// parties (me, friend), dated before cutoff — the archivable direct history.
func (s *Store) ListDirectCollapsible(ctx context.Context, me, friend int64, cutoff string) ([]*Expense, error) {
	conds := []string{
		"e.deleted_at IS NULL",
		"e.group_id IS NULL",
		"e.expense_date < ?",
		"e.id IN (SELECT expense_id FROM expense_participants WHERE user_id = ?)",
		"e.id IN (SELECT expense_id FROM expense_participants WHERE user_id = ?)",
		"(SELECT COUNT(*) FROM expense_participants ep WHERE ep.expense_id = e.id) = 2",
		notCollapsible,
	}
	return s.queryExpenses(ctx, conds, []any{cutoff, me, friend})
}

// ListGroupCollapsible returns a group's expenses dated before cutoff — the
// archivable group history.
func (s *Store) ListGroupCollapsible(ctx context.Context, groupID int64, cutoff string) ([]*Expense, error) {
	conds := []string{
		"e.deleted_at IS NULL",
		"e.group_id = ?",
		"e.expense_date < ?",
		notCollapsible,
	}
	return s.queryExpenses(ctx, conds, []any{groupID, cutoff})
}

// HistoricalBatch is one currency's collapse: the synthetic "Historical
// Transactions" expense (+ its zero-sum participants) that replaces OriginIDs.
type HistoricalBatch struct {
	Expense      *Expense
	Participants []ExpenseParticipant
	OriginIDs    []string
}

// CollapseToHistorical atomically, for each batch: inserts the synthetic expense
// and its participants, moves the origin expenses (+ participants) into the
// archive tables tagged with the synthetic id, and deletes the originals.
func (s *Store) CollapseToHistorical(ctx context.Context, batches []HistoricalBatch) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := nowISO()
	for _, b := range batches {
		if err := s.insertExpenseTx(ctx, tx, b.Expense, b.Participants); err != nil {
			return err
		}
		if len(b.OriginIDs) == 0 {
			continue
		}
		ph := inPlaceholders(len(b.OriginIDs))
		ids := make([]any, len(b.OriginIDs))
		for i, id := range b.OriginIDs {
			ids[i] = id
		}
		archArgs := append([]any{b.Expense.ID, now}, ids...)
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO archived_expenses (id, name, category, amount, split_type, expense_date,
				currency, paid_by, added_by, updated_by, group_id, file_key, transaction_id,
				recurrence_id, conversion_to_id, moved_from_id, deleted_at, deleted_by, created_at,
				updated_at, note, archive_expense_id, archived_at)
			 SELECT id, name, category, amount, split_type, expense_date, currency, paid_by, added_by,
				updated_by, group_id, file_key, transaction_id, recurrence_id, conversion_to_id,
				moved_from_id, deleted_at, deleted_by, created_at, updated_at, note, ?, ?
			 FROM expenses WHERE id IN (`+ph+`)`), archArgs...); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, s.rebind(
			`INSERT INTO archived_expense_participants (expense_id, user_id, amount)
			 SELECT expense_id, user_id, amount FROM expense_participants WHERE expense_id IN (`+ph+`)`),
			ids...); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, s.rebind(
			`DELETE FROM expenses WHERE id IN (`+ph+`)`), ids...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// inPlaceholders returns "?, ?, ..." with n placeholders (rebound per engine).
func inPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?, ", n-1) + "?"
}

func (s *Store) queryExpenses(ctx context.Context, conds []string, args []any) ([]*Expense, error) {
	q := `SELECT ` + expenseCols + ` FROM expenses e WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY e.expense_date DESC, e.created_at DESC`
	rows, err := s.DB.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Expense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
