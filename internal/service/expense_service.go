package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	// Version is the expense version the edit form was rendered from. Ignored on
	// add; on edit it must still match the stored row or the save is rejected as
	// stale (see store.ErrStaleExpense).
	Version int64
}

// SettlementInput is the editable surface of a settlement. The pair, the
// direction and the currency are fixed when it is recorded -- changing one means
// deleting it and recording it again -- so only the amount, the metadata and the
// target group can change here. Currency is still carried so a resubmitted form
// can be checked against the stored one rather than silently ignored.
type SettlementInput struct {
	Name        string
	Amount      int64
	Currency    string
	ExpenseDate string
	Note        string
	GroupID     *int64
	ActorID     int64
	Version     int64
}

// Errors surfaced to expense handlers.
var (
	// ErrNotMember names the actor too, not just the participants: see assertMembers.
	ErrNotMember       = errors.New("you and everyone involved must be members of the target group")
	ErrNotMovable      = errors.New("this expense cannot be moved")
	ErrMoveNoAck       = errors.New("you must acknowledge the recalculated split")
	ErrNotEditor       = errors.New("you can't edit or delete this expense")
	ErrExpenseNotFound = store.ErrNotFound
	ErrNotSettlement   = errors.New("this transaction is not a settlement")
	ErrIsSettlement    = errors.New("a settlement must be edited as a settlement")
	ErrBadSettlement   = errors.New("this settlement's rows are malformed and cannot be edited")
	// ErrSettlementAmount and ErrSettlementCurrency are sentinels rather than
	// bare errors so the handler can render them against the field that caused
	// them (see httpapp.fieldErrorsFor) instead of only as a banner.
	ErrSettlementAmount   = errors.New("settlement amount must be positive")
	ErrSettlementCurrency = errors.New("a settlement's currency is fixed; delete it and record it again in another currency")
)

// recordSplitInputs saves the raw per-participant inputs beside an expense so
// the edit form can restore exactly what was entered. Best-effort by design: the
// payload is reference data only, and when it is missing the form falls back to
// reconstructing values from the stored amounts, so a failure here must never
// fail the save the user already completed.
func (s *Service) recordSplitInputs(ctx context.Context, expenseID string, in ExpenseInput) {
	payload, err := split.EncodeInputs(in.Method, in.Lines)
	if err != nil || payload == "" {
		return
	}
	// Best-effort, but never silent: losing the payload downgrades the next edit
	// form to reconstructing values from the amounts, and nothing else would say
	// why.
	if err := s.Store.PutSplitInputs(ctx, expenseID, payload); err != nil {
		slog.Warn("split inputs not recorded; the edit form will fall back to derived values",
			"expense", expenseID, "err", err)
	}
}

// AddExpense computes the zero-sum participant rows via the split engine and
// persists the expense.
func (s *Service) AddExpense(ctx context.Context, in ExpenseInput) (*store.Expense, error) {
	if in.GroupID != nil {
		if err := s.assertMembers(ctx, *in.GroupID, in.Lines, in.PaidBy, in.ActorID); err != nil {
			return nil, err
		}
	}
	parts, err := s.buildParticipants(in)
	if err != nil {
		return nil, err
	}
	e := s.newExpense(in)
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil {
		return nil, err
	}
	s.recordSplitInputs(ctx, created.ID, in)
	// Ensure direct-expense counterparts become friends.
	if in.GroupID == nil {
		for _, p := range parts {
			if p.UserID != in.PaidBy {
				_ = s.Store.AddFriend(ctx, in.PaidBy, p.UserID)
			}
		}
	}
	s.notifyExpense(ctx, created, parts, in.ActorID, KindExpenseAdded)
	return created, nil
}

// CanEditExpense reports whether actorID is a member of the transaction — its
// payer, its creator, or one of its participants — and so may edit or delete it.
// Group membership alone is NOT enough: a group member who is not part of this
// expense can neither edit nor delete it.
func (s *Service) CanEditExpense(ctx context.Context, actorID int64, e *store.Expense) bool {
	if e.PaidBy == actorID || e.AddedBy == actorID {
		return true
	}
	parts, err := s.Store.GetParticipants(ctx, e.ID)
	if err != nil {
		return false
	}
	for _, p := range parts {
		if p.UserID == actorID {
			return true
		}
	}
	return false
}

// Transfer is one leg of a group settle-up: who pays whom, how much.
type Transfer struct {
	FromID   int64
	ToID     int64
	Amount   int64
	Currency string
}

// Settle records a settlement payment (sender pays receiver) as a SETTLEMENT
// expense driving balances toward zero.
func (s *Service) Settle(ctx context.Context, sender, receiver int64, amount int64, currency string, groupID *int64, date string, actor int64) (*store.Expense, error) {
	created, parts, err := s.settle(ctx, sender, receiver, amount, currency, groupID, date, actor)
	if err != nil {
		return nil, err
	}
	s.notifyExpense(ctx, created, parts, actor, KindSettlementAdded)
	return created, nil
}

// SettleAll records a group settle-up: every transfer becomes its own settlement
// and its own notification rows, but the batch produces a single push per
// recipient. It owns the loop that used to live in the handler, so a settle-up
// is one reported outcome rather than a half-applied set behind an error page.
func (s *Service) SettleAll(ctx context.Context, transfers []Transfer, groupID int64,
	date string, actor int64) ([]*store.Expense, error) {
	settled := make([]*store.Expense, 0, len(transfers))
	partsByID := make(map[string][]store.ExpenseParticipant, len(transfers))
	for _, t := range transfers {
		e, parts, err := s.settle(ctx, t.FromID, t.ToID, t.Amount, t.Currency, &groupID, date, actor)
		if err != nil {
			return settled, err
		}
		settled = append(settled, e)
		partsByID[e.ID] = parts
	}
	if len(settled) == 0 {
		return settled, nil
	}
	name := ""
	if g, err := s.Store.GetGroup(ctx, groupID); err == nil {
		name = g.Name
	}
	s.notifySettleBatch(ctx, settled, partsByID, actor, groupID, name)
	return settled, nil
}

// settle writes one settlement and returns it with its participant rows. It
// records but never notifies, so Settle can announce a single transfer while
// SettleAll coalesces a batch -- neither needs a mode flag on the other.
func (s *Service) settle(ctx context.Context, sender, receiver int64, amount int64,
	currency string, groupID *int64, date string, actor int64) (*store.Expense, []store.ExpenseParticipant, error) {
	if amount <= 0 {
		return nil, nil, ErrSettlementAmount
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
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil {
		return nil, nil, err
	}
	return created, parts, nil
}

// MoveExpense edits an expense in place (the shared move/edit action): the SAME
// record is updated with the recalculated split and note (no soft-delete, no
// duplicate). The actor must be authorized to edit it (payer/creator/participant/
// group member). The recalculated-split acknowledgment (acknowledged=true) is
// required ONLY when the expense is relocated to a different group — a plain
// in-place edit (same group, incl. direct→direct) needs no ack. Eligibility
// (§5.2) and target membership are validated. The actor must be a member of the
// transaction (payer/creator/participant) and, when a group is targeted, of that
// group. created_at/added_by are preserved; the note comes from the form
// (in.Note) and the receipt (file_key) is carried over since the form doesn't
// resubmit it. A settlement is refused outright: its two rows are hand-built,
// so re-splitting them here would rewrite split_type -- see UpdateSettlement.
func (s *Service) MoveExpense(ctx context.Context, origID string, in ExpenseInput, acknowledged bool) (*store.Expense, error) {
	orig, err := s.Store.GetExpense(ctx, origID)
	if err != nil {
		return nil, err
	}
	if orig.SplitType == string(split.SETTLEMENT) {
		return nil, ErrIsSettlement
	}
	if !s.CanEditExpense(ctx, in.ActorID, orig) {
		return nil, ErrNotEditor
	}
	if !movable(orig) {
		return nil, ErrNotMovable
	}
	if !sameGroup(orig.GroupID, in.GroupID) && !acknowledged {
		return nil, ErrMoveNoAck
	}
	if in.GroupID != nil {
		if err := s.assertMembers(ctx, *in.GroupID, in.Lines, in.PaidBy, in.ActorID); err != nil {
			return nil, err
		}
	}
	parts, err := s.buildParticipants(in)
	if err != nil {
		return nil, err
	}
	e := s.newExpense(in)
	e.ID = origID            // edit the original row in place
	e.FileKey = orig.FileKey // preserve the receipt (not resubmitted by the move form)
	e.Version = in.Version   // reject the save if someone else edited it meanwhile
	if err := s.Store.UpdateExpense(ctx, e, parts); err != nil {
		return nil, err
	}
	s.recordSplitInputs(ctx, origID, in)
	updated, err := s.Store.GetExpense(ctx, origID)
	if err != nil {
		return nil, err
	}
	s.notifyExpense(ctx, updated, parts, in.ActorID, KindExpenseUpdated)
	return updated, nil
}

// UpdateSettlement edits a SETTLEMENT in place, preserving its split type and
// its two zero-sum rows (payer +amount, counterparty -amount).
//
// Settlements get their own path deliberately: their rows are hand-built by
// Settle and never pass through the split engine, so routing an edit through the
// ordinary expense form would rewrite split_type and silently turn the record
// into a normal expense -- breaking the balance UI and every settlement query.
// The pair, the direction and the currency are fixed at creation; changing any
// of those means deleting the settlement and recording it again. Relocating to
// another group needs the same acknowledgment as any other move, because it
// changes which balance the settlement clears.
func (s *Service) UpdateSettlement(ctx context.Context, id string, in SettlementInput, acknowledged bool) (*store.Expense, error) {
	orig, err := s.Store.GetExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	if orig.SplitType != string(split.SETTLEMENT) {
		return nil, ErrNotSettlement
	}
	if !s.CanEditExpense(ctx, in.ActorID, orig) {
		return nil, ErrNotEditor
	}
	if !movable(orig) {
		return nil, ErrNotMovable
	}
	if in.Amount <= 0 {
		return nil, ErrSettlementAmount
	}
	// A settlement's currency picks which per-currency balance it clears, so
	// switching it would leave the original debt un-offset and invent a second
	// one in the new currency. It is fixed for the life of the record; an empty
	// value means the form carried none, which keeps the stored one.
	currency := strings.ToUpper(orDefault(in.Currency, orig.Currency))
	if currency != orig.Currency {
		return nil, ErrSettlementCurrency
	}
	if !sameGroup(orig.GroupID, in.GroupID) && !acknowledged {
		return nil, ErrMoveNoAck
	}
	payer, other, err := s.SettlementParties(ctx, orig)
	if err != nil {
		return nil, err
	}
	if in.GroupID != nil {
		// The actor is checked alongside the pair, not just the pair: settling a
		// whole group makes the acting member added_by on transfers between two
		// other people, and CanEditExpense then lets them edit it -- without this
		// they could file that settlement into a group they do not belong to.
		if err := s.assertGroupMembers(ctx, *in.GroupID, payer, other, in.ActorID); err != nil {
			return nil, err
		}
	}
	e := &store.Expense{
		ID:          orig.ID,
		Name:        orDefault(in.Name, orig.Name),
		Category:    orig.Category,
		Amount:      in.Amount,
		SplitType:   orig.SplitType,
		ExpenseDate: orDefault(in.ExpenseDate, orig.ExpenseDate),
		Currency:    currency,
		PaidBy:      payer,
		AddedBy:     orig.AddedBy,
		UpdatedBy:   sql.NullInt64{Int64: in.ActorID, Valid: true},
		GroupID:     nullInt(in.GroupID),
		FileKey:     orig.FileKey,
		Note:        in.Note,
		Version:     in.Version,
	}
	parts := []store.ExpenseParticipant{
		{UserID: payer, Amount: in.Amount},
		{UserID: other, Amount: -in.Amount},
	}
	if err := s.Store.UpdateExpense(ctx, e, parts); err != nil {
		return nil, err
	}
	updated, err := s.Store.GetExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	s.notifyExpense(ctx, updated, parts, in.ActorID, KindSettlementUpdated)
	return updated, nil
}

// SettlementParties resolves a settlement's payer and the person they paid from
// its stored rows -- who owes whom is not recoverable from the expense row alone.
// A settlement is exactly two rows by construction (see Settle); anything else is
// a corrupt record, and editing it would have to guess at the direction, so it
// fails loudly rather than silently picking one.
func (s *Service) SettlementParties(ctx context.Context, e *store.Expense) (payer, other int64, err error) {
	parts, err := s.Store.GetParticipants(ctx, e.ID)
	if err != nil {
		return 0, 0, err
	}
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w (%d participant rows, want 2)", ErrBadSettlement, len(parts))
	}
	for _, p := range parts {
		if p.UserID == e.PaidBy {
			payer = p.UserID
		} else {
			other = p.UserID
		}
	}
	if payer == 0 || other == 0 {
		return 0, 0, fmt.Errorf("%w (payer %d is not among its rows)", ErrBadSettlement, e.PaidBy)
	}
	return payer, other, nil
}

// DeleteExpense soft-deletes an expense after confirming the actor is a member of
// the transaction (payer/creator/participant). Deleting a settlement is the
// supported way to reverse it: the row leaves balance_view (deleted_at IS NOT
// NULL) and the balance is restored.
//
// version is the expense version the delete button was rendered from, and it is
// asserted like any other write: a delete decided from a stale page -- one that
// never showed the edit someone else has since saved -- is rejected with
// store.ErrStaleExpense instead of discarding a split the deleter never saw.
func (s *Service) DeleteExpense(ctx context.Context, id string, actorID, version int64) error {
	e, err := s.Store.GetExpense(ctx, id)
	if err != nil {
		return err // ErrExpenseNotFound
	}
	if !s.CanEditExpense(ctx, actorID, e) {
		return ErrNotEditor
	}
	// Read the participants before the delete: they are who gets told, and a
	// failure here must not stop a delete the actor is entitled to make. A soft
	// delete leaves expense_participants intact either way.
	parts, pErr := s.Store.GetParticipants(ctx, id)
	if err := s.Store.SoftDeleteExpense(ctx, id, actorID, version); err != nil {
		return err
	}
	if pErr != nil {
		slog.Warn("notify: participants for deleted expense", "expense", id, "err", pErr)
		return nil
	}
	s.notifyExpense(ctx, e, parts, actorID, KindExpenseDeleted)
	return nil
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

// assertMembers requires every user the save would touch -- the payer, each
// participant, and the actor performing it -- to belong to the target group.
// The actor is included because being a member of the transaction is not the
// same as being a member of the destination: creator rights on a transfer
// between two other people would otherwise let someone file it into a group
// they have no part in, where it would move that group's balances.
func (s *Service) assertMembers(ctx context.Context, groupID int64, lines []split.Line, payer, actor int64) error {
	ids := map[int64]bool{payer: true, actor: true}
	for _, l := range lines {
		ids[l.UserID] = true
	}
	uniq := make([]int64, 0, len(ids))
	for id := range ids {
		uniq = append(uniq, id)
	}
	return s.assertGroupMembers(ctx, groupID, uniq...)
}

// assertGroupMembers fails closed with ErrNotMember unless every id belongs to
// groupID; a lookup failure is reported as-is rather than treated as absence.
func (s *Service) assertGroupMembers(ctx context.Context, groupID int64, ids ...int64) error {
	for _, id := range ids {
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
