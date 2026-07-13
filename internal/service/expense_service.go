package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// ExpenseInput is the normalized form data for creating/editing an expense.
type ExpenseInput struct {
	Name        string
	Category    string
	Total       int64 // minor units
	Method      split.Method
	Currency    string
	ExpenseDate string // ISO date
	PaidBy      int64
	GroupID     *int64 // nil = direct/non-group expense
	FileKey     string
	Note        string       // free-text note (edit form)
	Lines       []split.Line // participants + method params
	ActorID     int64        // who is performing the action
}

// Errors surfaced to expense handlers.
var (
	ErrNotMember       = errors.New("all participants must be members of the target group")
	ErrNotMovable      = errors.New("this expense cannot be moved")
	ErrMoveNoAck       = errors.New("you must acknowledge the recalculated split")
	ErrNotEditor       = errors.New("you can't edit this expense")
	ErrExpenseNotFound = store.ErrNotFound
)

// AddExpense computes the zero-sum participant rows via the split engine and
// persists the expense.
func (s *Service) AddExpense(ctx context.Context, in ExpenseInput) (*store.Expense, error) {
	parts, err := s.buildParticipants(in)
	if err != nil {
		return nil, err
	}
	e := s.newExpense(in)
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil {
		return nil, err
	}
	// Ensure direct-expense counterparts become friends.
	if in.GroupID == nil {
		for _, p := range parts {
			if p.UserID != in.PaidBy {
				_ = s.Store.AddFriend(ctx, in.PaidBy, p.UserID)
			}
		}
	}
	s.notifyExpense(created, parts, in.ActorID, "added")
	return created, nil
}

// canEditExpense reports whether actor may edit an expense: payer/creator, a
// participant, or a member of its group.
func (s *Service) canEditExpense(ctx context.Context, actorID int64, e *store.Expense) bool {
	if e.PaidBy == actorID || e.AddedBy == actorID {
		return true
	}
	if parts, err := s.Store.GetParticipants(ctx, e.ID); err == nil {
		for _, p := range parts {
			if p.UserID == actorID {
				return true
			}
		}
	}
	if e.GroupID.Valid {
		if ok, _ := s.Store.IsGroupMember(ctx, e.GroupID.Int64, actorID); ok {
			return true
		}
	}
	return false
}

// Settle records a settlement payment (sender pays receiver) as a SETTLEMENT
// expense driving balances toward zero.
func (s *Service) Settle(ctx context.Context, sender, receiver int64, amount int64, currency string, groupID *int64, date string, actor int64) (*store.Expense, error) {
	if amount <= 0 {
		return nil, errors.New("settlement amount must be positive")
	}
	e := &store.Expense{
		ID:          store.NewUUID(),
		Name:        "Settlement",
		Category:    "settlement",
		Amount:      amount,
		SplitType:   string(split.SETTLEMENT),
		ExpenseDate: date,
		Currency:    strings.ToUpper(currency),
		PaidBy:      sender,
		AddedBy:     actor,
		GroupID:     nullInt(groupID),
	}
	parts := []store.ExpenseParticipant{
		{UserID: sender, Amount: amount},
		{UserID: receiver, Amount: -amount},
	}
	return s.Store.CreateExpense(ctx, e, parts)
}

// MoveExpense edits an expense in place (the shared move/edit action): the SAME
// record is updated with the recalculated split and note (no soft-delete, no
// duplicate). The actor must be authorized to edit it (payer/creator/participant/
// group member). The recalculated-split acknowledgment (acknowledged=true) is
// required ONLY when the expense is relocated to a different group — a plain
// in-place edit (same group, incl. direct→direct) needs no ack. Eligibility
// (§5.2) and target membership are validated. created_at/added_by are preserved;
// the note comes from the form (in.Note) and the receipt (file_key) is carried
// over since the form doesn't resubmit it.
func (s *Service) MoveExpense(ctx context.Context, origID string, in ExpenseInput, acknowledged bool) (*store.Expense, error) {
	orig, err := s.Store.GetExpense(ctx, origID)
	if err != nil {
		return nil, err
	}
	if !s.canEditExpense(ctx, in.ActorID, orig) {
		return nil, ErrNotEditor
	}
	if !movable(orig) {
		return nil, ErrNotMovable
	}
	if !sameGroup(orig.GroupID, in.GroupID) && !acknowledged {
		return nil, ErrMoveNoAck
	}
	if in.GroupID != nil {
		if err := s.assertMembers(ctx, *in.GroupID, in.Lines, in.PaidBy); err != nil {
			return nil, err
		}
	}
	parts, err := s.buildParticipants(in)
	if err != nil {
		return nil, err
	}
	e := s.newExpense(in)
	e.ID = origID          // edit the original row in place
	e.FileKey = orig.FileKey // preserve the receipt (not resubmitted by the move form)
	if err := s.Store.UpdateExpense(ctx, e, parts); err != nil {
		return nil, err
	}
	return s.Store.GetExpense(ctx, origID)
}

// movable enforces §5.2 eligibility: currency-conversion pairs and recurrence
// templates are not movable; everything else (incl. settlements) is.
func movable(e *store.Expense) bool {
	if e.SplitType == string(split.CURRENCY_CONVERSION) || e.ConversionToID.Valid {
		return false
	}
	// Historical Transactions are a synthetic collapse of many rows — not movable.
	if e.SplitType == string(split.ARCHIVE) {
		return false
	}
	// Expenses generated by a recurrence move like any other; a recurrence
	// *template* carries no recurrence_id and is guarded at the handler layer.
	return !e.DeletedAt.Valid
}

func (s *Service) assertMembers(ctx context.Context, groupID int64, lines []split.Line, payer int64) error {
	ids := map[int64]bool{payer: true}
	for _, l := range lines {
		ids[l.UserID] = true
	}
	for id := range ids {
		ok, err := s.Store.IsGroupMember(ctx, groupID, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotMember
		}
	}
	return nil
}

func (s *Service) buildParticipants(in ExpenseInput) ([]store.ExpenseParticipant, error) {
	ps, err := split.Compute(in.Method, in.Total, in.PaidBy, in.ExpenseDate, in.Lines)
	if err != nil {
		return nil, err
	}
	out := make([]store.ExpenseParticipant, len(ps))
	for i, p := range ps {
		out[i] = store.ExpenseParticipant{UserID: p.UserID, Amount: p.Amount}
	}
	return out, nil
}

func (s *Service) newExpense(in ExpenseInput) *store.Expense {
	return &store.Expense{
		ID:          store.NewUUID(),
		Name:        in.Name,
		Category:    orDefault(in.Category, "general"),
		Amount:      in.Total,
		SplitType:   string(in.Method),
		ExpenseDate: in.ExpenseDate,
		Currency:    strings.ToUpper(in.Currency),
		PaidBy:      in.PaidBy,
		AddedBy:     in.ActorID,
		UpdatedBy:   sql.NullInt64{Int64: in.ActorID, Valid: true},
		GroupID:     nullInt(in.GroupID),
		FileKey:     nullStr(in.FileKey),
		Note:        in.Note,
	}
}

// sameGroup reports whether an expense's stored group (nullable) and a target
// group id (nil = no group) refer to the same group — used to decide whether a
// re-split acknowledgment is required. Both absent counts as the same group.
func sameGroup(orig sql.NullInt64, in *int64) bool {
	if !orig.Valid {
		return in == nil
	}
	return in != nil && *in == orig.Int64
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
